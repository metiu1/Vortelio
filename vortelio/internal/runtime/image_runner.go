package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vortelio/vortelio/internal/hub"
)

type ImageRunner struct {
	model *hub.Model
	hw    *Hardware
}

func NewImageRunner(model *hub.Model, hw *Hardware) *ImageRunner {
	return &ImageRunner{model: model, hw: hw}
}

func (r *ImageRunner) Run(opts *RunOptions) error {
	if opts.Prompt == "" {
		return fmt.Errorf("image generation requires a text prompt\n  Example: vortelio run image/sdxl \"a cat eating pasta\"")
	}

	output := ResolveOutputPath(opts.OutputFile, "output.png")
	device := r.deviceString(opts.ForceCPU)

	fmt.Printf("🎨  Generating image (%d step, device=%s)\n", opts.Steps, device)
	fmt.Printf("    Prompt: %q\n", opts.Prompt)
	fmt.Printf("    Output: %s\n\n", output)

	modelExt := strings.ToLower(filepath.Ext(r.model.LocalPath))
	isGGUF := modelExt == ".gguf"

	if isGGUF {
		// GGUF: native SD.cpp doesn't need Python; FindPython() used only as fallback
		return r.runGGUF(FindPython(), opts.Prompt, output, device, opts.Steps, opts.ForceCPU)
	}

	pythonBin := FindPython()
	if pythonBin == "" {
		fmt.Println("\n⚠️   Python 3 not found.")
		fmt.Println("    Install Python 3.10+ from: https://python.org/downloads")
		return nil
	}
	return r.runDiffusers(pythonBin, opts.Prompt, output, device, opts.Steps)
}

// runGGUF runs a GGUF image model. Prefers the native stable-diffusion.cpp binary;
// falls back to the stable-diffusion-cpp-python package if the binary is not available.
func (r *ImageRunner) runGGUF(pythonBin, prompt, output, device string, steps int, forceCPU bool) error {
	// ── Try native sd binary first ───────────────────────────────────────────
	if sdBin := SDCppBin(); sdBin != "" {
		return r.runNativeSD(sdBin, prompt, output, steps, forceCPU)
	}

	// Not installed — offer to download
	fmt.Println("📦  stable-diffusion.cpp not found. Downloading the binary...")
	if err := InstallSDCpp(r.hw); err != nil {
		fmt.Printf("⚠️   Download failed (%v). Using the Python backend...\n", err)
	} else if sdBin := SDCppBin(); sdBin != "" {
		return r.runNativeSD(sdBin, prompt, output, steps, forceCPU)
	}

	// ── Fallback: stable-diffusion-cpp-python ───────────────────────────────
	if pythonBin == "" {
		return fmt.Errorf("neither stable-diffusion.cpp nor Python found\n  Download manually: https://github.com/leejet/stable-diffusion.cpp/releases")
	}
	if !CheckPythonPackage(pythonBin, "stable_diffusion_cpp") {
		fmt.Println("📦  Installing stable-diffusion-cpp-python (backend GGUF)...")
		_ = InstallPythonPackage(pythonBin, "stable-diffusion-cpp-python")
		if !CheckPythonPackage(pythonBin, "stable_diffusion_cpp") {
			fmt.Println("❌  Installation failed. Install manually:")
			fmt.Println("    pip install stable-diffusion-cpp-python")
			return nil
		}
	}

	modelPath := strings.ReplaceAll(r.model.LocalPath, `\`, `/`)
	output = strings.ReplaceAll(output, `\`, `/`)
	useCUDA := r.hw.Backend == BackendCUDA && !forceCPU

	script := fmt.Sprintf(`import sys, warnings, inspect
warnings.filterwarnings("ignore")
try:
    from stable_diffusion_cpp import StableDiffusion
except ImportError:
    print("Error: stable-diffusion-cpp-python not found.")
    print("  pip install stable-diffusion-cpp-python")
    sys.exit(1)

model_path = r'''%s'''
use_cuda   = %s

print("Loading model GGUF...")
n_gpu = -1 if use_cuda else 0

# Try construction — some versions need different params
try:
    sd = StableDiffusion(
        model_path=model_path,
        n_threads=-1,
        n_gpu_layers=n_gpu,
        verbose=True,  # show loading progress
    )
except Exception as e1:
    print(f"  Tentativo senza GPU: {e1}")
    try:
        sd = StableDiffusion(model_path=model_path, n_threads=-1, verbose=True)
    except Exception as e2:
        print(f"Model load error: {e2}")
        print()
        print("Possibili cause:")
        print("  - GGUF file corrupted or incomplete (retry vortelio pull)")
        print("  - Architecture not supported (try a safetensors model)")
        print("  - Versione stable-diffusion-cpp-python incompatibile")
        print("    pip install --upgrade stable-diffusion-cpp-python")
        sys.exit(1)

prompt_text = '''%s'''
print("Generating...")

# stable_diffusion_cpp 0.4+: generate_image(prompt, negative_prompt, ...)
# older: txt_to_img(prompt, negative_prompt, ...)
img = None
gen_kwargs = dict(
    negative_prompt="low quality, blurry, deformed",
    cfg_scale=7.0,
    sample_steps=%d,
    width=512,
    height=512,
    seed=-1,
)
if hasattr(sd, "generate_image"):
    r = sd.generate_image(prompt_text, **gen_kwargs)
    img = r[0] if isinstance(r, (list,tuple)) else r
elif hasattr(sd, "txt_to_img"):
    r = sd.txt_to_img(prompt_text, **gen_kwargs)
    img = r[0] if isinstance(r, (list,tuple)) else r
elif hasattr(sd, "text_to_image"):
    r = sd.text_to_image(prompt_text, **gen_kwargs)
    img = r[0] if isinstance(r, (list,tuple)) else r
else:
    gen_methods = [m for m in dir(sd) if not m.startswith("_") and ("img" in m or "image" in m or "gen" in m)]
    print(f"Error: API not recognized. Methods found: {gen_methods}")
    print("Aggiorna il pacchetto: pip install --upgrade stable-diffusion-cpp-python")
    sys.exit(1)

if img is None:
    print("Error: no image generated.")
    sys.exit(1)

img.save(r'''%s''')
print(f"\n\u2705  Image saved: %s")
`, modelPath, boolPy(useCUDA), escapePy(prompt), steps, output, output)
	return r.runPython(pythonBin, script)
}

// runNativeSD runs generation using the native stable-diffusion.cpp binary.
func (r *ImageRunner) runNativeSD(sdBin, prompt, output string, steps int, forceCPU bool) error {
	args := []string{
		"-m", r.model.LocalPath,
		"-p", prompt,
		"-n", "low quality, blurry, deformed, bad anatomy, extra limbs",
		"--steps", strconv.Itoa(steps),
		"--cfg-scale", "7.0",
		"-W", "512",
		"-H", "512",
		"-s", "-1",
		"-o", output,
	}

	// GPU layers: -1 = all layers on GPU, 0 = CPU only
	if !forceCPU && (r.hw.Backend == BackendCUDA || r.hw.Backend == BackendMetal) {
		args = append(args, "--n-gpu-layers", "-1")
	}

	cmd := HideWindow(exec.Command(sdBin, args...))
	if err := RunWithOutput(cmd, os.Stdout, os.Stderr); err != nil {
		return fmt.Errorf("stable-diffusion.cpp: %w", err)
	}
	return nil
}

// runDiffusers handles safetensors/diffusers format models
func (r *ImageRunner) runDiffusers(pythonBin, prompt, output, device string, steps int) error {
	if !CheckPythonPackage(pythonBin, "diffusers") {
		fmt.Println("📦  diffusers not installed.")
		if r.hw.Backend == BackendCUDA {
			fmt.Println("    pip install diffusers transformers accelerate torch --index-url https://download.pytorch.org/whl/cu121")
		} else {
			fmt.Println("    pip install diffusers transformers accelerate torch")
		}
		return nil
	}

	modelPath := r.model.LocalPath
	modelDir := filepath.Dir(modelPath)
	for {
		if _, err := os.Stat(filepath.Join(modelDir, "model_index.json")); err == nil {
			break
		}
		p := filepath.Dir(modelDir)
		if p == modelDir {
			break
		}
		modelDir = p
	}

	modelPath = strings.ReplaceAll(modelPath, `\`, `/`)
	modelDir = strings.ReplaceAll(modelDir, `\`, `/`)
	output = strings.ReplaceAll(output, `\`, `/`)

	script := fmt.Sprintf(`import sys, os, warnings
warnings.filterwarnings("ignore")
try:
    import torch
    from diffusers import DiffusionPipeline, StableDiffusionPipeline, StableDiffusionXLPipeline
except ImportError as e:
    print(f"Dipendenza mancante: {e}")
    print("pip install diffusers transformers accelerate torch")
    sys.exit(1)

device     = %q
model_path = r'''%s'''
model_dir  = r'''%s'''
dtype = torch.float16 if device != "cpu" else torch.float32

print("Loading model...")
pipe = None
has_index = os.path.exists(os.path.join(model_dir, "model_index.json"))

if has_index:
    try:
        pipe = DiffusionPipeline.from_pretrained(model_dir, torch_dtype=dtype, safety_checker=None)
    except Exception as e:
        print(f"  from_pretrained: {e}")

if pipe is None and os.path.isfile(model_path):
    for cls in [StableDiffusionXLPipeline, StableDiffusionPipeline, DiffusionPipeline]:
        try:
            pipe = cls.from_single_file(model_path, torch_dtype=dtype, safety_checker=None)
            break
        except Exception:
            continue

if pipe is None:
    print("Error: could not load the model.")
    sys.exit(1)

pipe = pipe.to(device)
if device != "cpu":
    pipe.enable_attention_slicing()
    try: pipe.enable_xformers_memory_efficient_attention()
    except: pass

print("Generating...")
result = pipe(prompt='''%s''', num_inference_steps=%d, guidance_scale=7.5)
result.images[0].save(r'''%s''')
print(f"\n✅  Image saved: %s")
`, device, modelPath, modelDir, escapePy(prompt), steps, output, output)

	return r.runPython(pythonBin, script)
}

func (r *ImageRunner) runPython(pythonBin, script string) error {
	tmp, err := os.CreateTemp("", "vortelio-img-*.py")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	cmd := HideWindow(exec.Command(pythonBin, tmp.Name()))
	cmd.Env = append(os.Environ(),
		"PYTHONIOENCODING=utf-8", "PYTHONUTF8=1",
		"USE_TF=0", "USE_JAX=0",
		"TRANSFORMERS_VERBOSITY=error",
		"TF_CPP_MIN_LOG_LEVEL=3",
	)
	if err := RunWithOutput(cmd, os.Stdout, os.Stderr); err != nil {
		return fmt.Errorf("image generation failed: %w", err)
	}
	return nil
}

// RunWithProgress runs image generation streaming progress events via channel.
// Since our image scripts already emit VORTELIO_PROGRESS lines, we run them
// through RunWithProgress which parses those lines into ProgressEvents.
func (r *ImageRunner) RunWithProgress(opts *RunOptions, progress chan<- ProgressEvent) error {
	if opts.Prompt == "" {
		if progress != nil {
			close(progress)
		}
		return fmt.Errorf("image generation requires a text prompt")
	}
	// Run normally but use RunCapture path which uses RunWithCapture for detailed errors
	// For progress: just signal start and done via channel workaround
	if progress != nil {
		progress <- ProgressEvent{Percent: 5, Message: "Starting generation..."}
	}
	err := r.Run(opts)
	if progress != nil {
		if err == nil {
			progress <- ProgressEvent{Percent: 100, Message: "Done!"}
		}
		close(progress)
	}
	return err
}

// GenerateToBytes runs image generation and returns the PNG bytes.
func (r *ImageRunner) GenerateToBytes(prompt string, steps int, forceCPU bool) ([]byte, error) {
	tmp, err := os.CreateTemp("", "vortelio-img-*.png")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	opts := &RunOptions{Prompt: prompt, OutputFile: tmpPath, Steps: steps, ForceCPU: forceCPU}
	if err := r.Run(opts); err != nil {
		return nil, err
	}
	return os.ReadFile(tmpPath)
}

func (r *ImageRunner) deviceString(forceCPU bool) string {
	if forceCPU {
		return "cpu"
	}
	switch r.hw.Backend {
	case BackendCUDA:
		return "cuda"
	case BackendMetal:
		return "mps"
	default:
		return "cpu"
	}
}

func boolPy(b bool) string {
	if b {
		return "True"
	}
	return "False"
}
