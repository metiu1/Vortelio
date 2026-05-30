package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vortelio/vortelio/internal/hub"
	internalruntime "github.com/vortelio/vortelio/internal/runtime"
)

// QuantizeCommand quantizes a model to a smaller format locally.
type QuantizeCommand struct{}

func NewQuantizeCommand() *QuantizeCommand { return &QuantizeCommand{} }

func (c *QuantizeCommand) Name() string { return "quantize" }

func (c *QuantizeCommand) Run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		c.printUsage()
		return nil
	}

	// Parse flags
	var modelRef, outputFormat, outputPath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format", "-f":
			if i+1 < len(args) {
				outputFormat = args[i+1]
				i++
			}
		case "--output", "-o":
			if i+1 < len(args) {
				outputPath = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "--") && modelRef == "" {
				modelRef = args[i]
			}
		}
	}

	if modelRef == "" {
		c.printUsage()
		return nil
	}

	// If no format given and model is GGUF, show interactive menu after resolving
	_ = outputFormat != "" // formatGiven (unused)
	if outputFormat == "" {
		outputFormat = "" // will be prompted in requantizeGGUF if needed
	}

	// Resolve model
	store := hub.NewModelStore()
	ref, err := hub.ParseModelRef(modelRef)
	if err != nil {
		return fmt.Errorf("invalid model reference: %w", err)
	}
	model, err := store.Resolve(ref)
	if err != nil {
		return fmt.Errorf("model not found: %w\n  Use 'vortelio list' to see installed models", err)
	}

	modelType := strings.ToLower(model.Type)
	switch modelType {
	case "llm":
		return c.quantizeLLM(model, outputFormat, outputPath)
	case "image":
		return c.quantizeImage(model, outputFormat, outputPath)
	default:
		return fmt.Errorf("quantization supported only for: llm, image\nCurrent type: %s", modelType)
	}
}

// quantizeLLM converts safetensors/bin LLMs to GGUF using llama.cpp's convert + quantize
func (c *QuantizeCommand) quantizeLLM(model *hub.Model, format, outputPath string) error {
	modelPath := model.LocalPath
	modelDir := filepath.Dir(modelPath)

	// If it's already GGUF, just quantize to a different level
	isGGUF := strings.HasSuffix(strings.ToLower(modelPath), ".gguf")
	if isGGUF {
		return c.requantizeGGUF(modelPath, format, outputPath)
	}

	// safetensors → GGUF: need convert_hf_to_gguf.py from llama.cpp
	fmt.Printf("📦  Quantizing LLM: %s/%s:%s\n", model.Type, model.Name, model.Tag)
	fmt.Printf("    Output format: %s\n", format)
	fmt.Println()

	// Check for llama.cpp convert script
	llamaDir := c.findLlamaDir()
	if llamaDir == "" {
		fmt.Println("⚠️   llama.cpp not found. Install with:")
		fmt.Println("    vortelio setup")
		fmt.Println()
		fmt.Println("    Or use Python with llama-cpp-python:")
		fmt.Println("    pip install llama-cpp-python")
		return c.quantizeLLMPython(model, format, outputPath)
	}

	// Run convert_hf_to_gguf.py
	convertScript := filepath.Join(llamaDir, "convert_hf_to_gguf.py")
	if _, err := os.Stat(convertScript); os.IsNotExist(err) {
		convertScript = filepath.Join(llamaDir, "convert.py")
	}

	outFile := outputPath
	if outFile == "" {
		outFile = filepath.Join(filepath.Dir(modelDir), model.Name+"-"+format+".gguf")
	}

	fmt.Printf("    Converting: %s → %s\n\n", modelDir, outFile)
	cmd := exec.Command("python3", convertScript,
		modelDir,
		"--outfile", outFile,
		"--outtype", strings.ToLower(format),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	fmt.Printf("\n✅  Model quantized: %s\n", outFile)
	fmt.Printf("    Import with: vortelio pull --file %s llm\n", outFile)
	return nil
}

// validQuantFormats lists all formats llama-quantize supports
var validQuantFormats = []string{
	"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L",
	"Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M",
	"Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M",
	"Q6_K", "Q8_0", "F16", "BF16",
	"IQ1_S", "IQ1_M", "IQ2_XXS", "IQ2_XS", "IQ2_S", "IQ2_M",
	"IQ3_XXS", "IQ3_XS", "IQ3_S", "IQ3_M",
	"IQ4_NL", "IQ4_XS",
}

// sourceCanBeQuantized returns true only if the source GGUF type supports requantization.
// llama-quantize only supports F16, BF16, F32 as source. Q3/Q4/Q5/Q6/Q8 cannot be re-quantized.
func sourceCanBeQuantized(inputPath string) (bool, string) {
	lower := strings.ToLower(filepath.Base(inputPath))
	// Detect existing quantization from filename
	quantTypes := []string{"q2_k", "q3_k", "q4_0", "q4_1", "q4_k", "q5_0", "q5_1", "q5_k", "q6_k", "q8_0", "iq1", "iq2", "iq3", "iq4"}
	for _, qt := range quantTypes {
		if strings.Contains(lower, qt) {
			return false, strings.ToUpper(qt)
		}
	}
	// Check for F16/BF16/F32 in name (these can be quantized)
	if strings.Contains(lower, "f16") || strings.Contains(lower, "fp16") ||
		strings.Contains(lower, "f32") || strings.Contains(lower, "fp32") ||
		strings.Contains(lower, "bf16") {
		return true, "F16/F32"
	}
	// Unknown — let llama-quantize decide (will fail if not supported)
	return false, "unknown"
}

// promptQuantFormat shows a menu and returns the chosen format
func promptQuantFormat(current string) string {
	fmt.Println()
	fmt.Println("Choose quantization level:")
	fmt.Println()
	fmt.Println("  Low quality / small file:")
	fmt.Println("    [1] IQ2_XXS  ~2.5 bpw  — absolute minimum")
	fmt.Println("    [2] Q2_K     ~2.6 bpw  — highly compressed")
	fmt.Println("    [3] IQ3_XS   ~3.3 bpw  — small with good quality")
	fmt.Println("    [4] Q3_K_M   ~3.5 bpw  — balanced small")
	fmt.Println()
	fmt.Println("  Medium quality / recommended:")
	fmt.Println("    [5] Q4_K_S   ~4.1 bpw  — great ratio")
	fmt.Println("    [6] Q4_K_M   ~4.5 bpw  — ⭐ DEFAULT recommended")
	fmt.Println("    [7] Q5_K_M   ~5.5 bpw  — high quality")
	fmt.Println("    [8] Q6_K     ~6.6 bpw  — near lossless")
	fmt.Println()
	fmt.Println("  High quality / large file:")
	fmt.Println("    [9] Q8_0     ~8.5 bpw  — maximum quantized quality")
	fmt.Println("   [10] F16      ~16  bpw  — no quantization")
	fmt.Println()
	if current != "" {
		fmt.Printf("  Current format: %s\n", current)
		fmt.Println("  (Press Enter to use default Q4_K_M)")
		fmt.Println()
	}
	fmt.Print("Choice [1-10] or direct name (e.g. Q5_K_M): ")
	var input string
	fmt.Fscan(os.Stdin, &input)
	input = strings.TrimSpace(input)
	if input == "" {
		return "Q4_K_M"
	}
	switch input {
	case "1":
		return "IQ2_XXS"
	case "2":
		return "Q2_K"
	case "3":
		return "IQ3_XS"
	case "4":
		return "Q3_K_M"
	case "5":
		return "Q4_K_S"
	case "6":
		return "Q4_K_M"
	case "7":
		return "Q5_K_M"
	case "8":
		return "Q6_K"
	case "9":
		return "Q8_0"
	case "10":
		return "F16"
	default:
		// User typed format name directly - validate it
		upper := strings.ToUpper(input)
		for _, f := range validQuantFormats {
			if strings.ToUpper(f) == upper {
				return upper
			}
		}
		fmt.Printf("⚠️   Format %q not recognized, using Q4_K_M\n", input)
		return "Q4_K_M"
	}
}

// requantizeGGUF re-quantizes an existing GGUF to a different quantization level
func (c *QuantizeCommand) requantizeGGUF(inputPath, format, outputPath string) error {
	// Detect source type and warn if it can't be re-quantized
	canQuantize, srcType := sourceCanBeQuantized(inputPath)
	if !canQuantize {
		fmt.Println()
		fmt.Printf("⚠️   WARNING: source file is already quantized (%s)\n", srcType)
		fmt.Println()
		fmt.Println("   llama-quantize does not support re-quantization from an already")
		fmt.Println("   compressed format. Quantization only works from F16/BF16/F32.")
		fmt.Println()
		fmt.Println("   OPTIONS:")
		fmt.Println("   1. Download the model in F16 format from HuggingFace:")
		fmt.Printf("      vortelio pull llm/hf.co/<owner>/<repo>:F16.gguf\n")
		fmt.Println()
		fmt.Println("   2. Use the model as-is (already quantized is optimal for your hardware)")
		fmt.Println()
		fmt.Println("   3. Force attempt anyway (will likely fail):")
		fmt.Print("      Proceed anyway? [y/N]: ")
		var resp string
		fmt.Fscan(os.Stdin, &resp)
		if strings.ToLower(strings.TrimSpace(resp)) != "s" {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	// If format not specified, show interactive menu
	if format == "" || format == "Q4_K_M" {
		// Check if user explicitly passed --format, if not show menu
		format = promptQuantFormat(srcType)
	}

	fmt.Println()
	fmt.Printf("🔄  GGUF quantization\n")
	fmt.Printf("    Input:  %s\n", inputPath)
	fmt.Printf("    Output format: %s\n", format)

	outFile := outputPath
	if outFile == "" {
		base := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
		for _, q := range []string{"Q8_0", "Q6_K", "Q5_K_M", "Q5_K_S", "Q4_K_M", "Q4_K_S", "Q4_0", "Q3_K_M", "Q3_K_S", "Q2_K", "IQ4_XS", "IQ3_XXS", "IQ2_XXS", "F16", "BF16"} {
			base = strings.TrimSuffix(base, "-"+q)
			base = strings.TrimSuffix(base, "_"+q)
			base = strings.TrimSuffix(base, "-"+strings.ToLower(q))
			base = strings.TrimSuffix(base, "_"+strings.ToLower(q))
		}
		outFile = base + "-" + format + ".gguf"
	}
	fmt.Printf("    Output: %s\n\n", outFile)

	quantizeBin := c.findLlamaQuantize()
	if quantizeBin != "" {
		cmd := exec.Command(quantizeBin, inputPath, outFile, format)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("quantization failed: %w\n\nHint: llama-quantize requires F16/F32 source. Download the F16 version of the model.", err)
		}
	} else {
		return c.requantizeGGUFPython(inputPath, format, outFile)
	}

	fmt.Printf("\n✅  Quantizzato: %s\n", outFile)
	fmt.Printf("    Size: %.2f GB\n", func() float64 {
		if fi, err := os.Stat(outFile); err == nil {
			return float64(fi.Size()) / 1e9
		}
		return 0
	}())
	return nil
}

func (c *QuantizeCommand) quantizeLLMPython(model *hub.Model, format, outputPath string) error {
	pythonBin := internalruntime.FindPython()
	if pythonBin == "" {
		return fmt.Errorf("Python not found")
	}

	modelDir := strings.ReplaceAll(filepath.Dir(model.LocalPath), `\`, `/`)
	outFile := outputPath
	if outFile == "" {
		outFile = filepath.Join(filepath.Dir(filepath.Dir(model.LocalPath)), model.Name+"-"+format+".gguf")
	}
	outFile = strings.ReplaceAll(outFile, `\`, `/`)

	script := fmt.Sprintf(`import sys
try:
    from llama_cpp import llama_model_quantize_params, llama_model_quantize
except ImportError:
    print("pip install llama-cpp-python")
    print("Or use: https://github.com/ggerganov/llama.cpp")
    sys.exit(1)
print("Quantization via llama-cpp-python not yet supported.")
print("Use directly: llama-quantize input.gguf output.gguf %s")
sys.exit(1)
`, format)

	tmp, _ := os.CreateTemp("", "vortelio-q-*.py")
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	cmd := exec.Command(pythonBin, tmp.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	_ = modelDir
	_ = outFile
	return nil
}

func (c *QuantizeCommand) requantizeGGUFPython(inputPath, format, outputPath string) error {
	pythonBin := internalruntime.FindPython()
	if pythonBin == "" {
		return fmt.Errorf("Python not found. Installa llama.cpp per requantizzare GGUF")
	}

	inputPath = strings.ReplaceAll(inputPath, `\`, `/`)
	outputPath = strings.ReplaceAll(outputPath, `\`, `/`)

	script := fmt.Sprintf(`import sys, subprocess, os
input_path  = r'''%s'''
output_path = r'''%s'''
fmt_str     = %q

# Try gguf-py approach
try:
    import gguf
    print("Requantization via gguf-py...")
    # gguf-py doesn't support requantization directly
    # Need llama-quantize binary
    raise NotImplementedError("gguf-py does not support requantization")
except (ImportError, NotImplementedError):
    pass

# Try finding llama-quantize in common locations
import shutil
lq = shutil.which("llama-quantize") or shutil.which("llama-quantize.exe")
if lq:
    print(f"llama-quantize found: {lq}")
    r = subprocess.run([lq, input_path, output_path, fmt_str], check=True)
    print(f"\n✅  Requantized: {output_path}")
    sys.exit(0)

print("❌  llama-quantize not found.")
print()
print("Install llama.cpp and add it to PATH:")
print("  https://github.com/ggerganov/llama.cpp/releases")
print()
print("Or build it locally:")
print("  git clone https://github.com/ggerganov/llama.cpp")
print("  cd llama.cpp && cmake -B build && cmake --build build")
print("  build/bin/llama-quantize", input_path, output_path, fmt_str)
sys.exit(1)
`, inputPath, outputPath, format)

	tmp, _ := os.CreateTemp("", "vortelio-q-*.py")
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	cmd := exec.Command(pythonBin, tmp.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// quantizeImage converts image model safetensors to GGUF via stable-diffusion.cpp
func (c *QuantizeCommand) quantizeImage(model *hub.Model, format, outputPath string) error {
	fmt.Printf("📦  Quantizing image model: %s/%s:%s\n", model.Type, model.Name, model.Tag)
	fmt.Printf("    Format: %s\n", format)
	fmt.Println()

	modelPath := strings.ReplaceAll(model.LocalPath, `\`, `/`)
	outFile := outputPath
	if outFile == "" {
		base := strings.TrimSuffix(model.LocalPath, filepath.Ext(model.LocalPath))
		outFile = base + "-" + format + ".gguf"
	}
	outFile = strings.ReplaceAll(outFile, `\`, `/`)

	fmt.Printf("    Input:  %s\n", model.LocalPath)
	fmt.Printf("    Output: %s\n\n", outFile)

	pythonBin := internalruntime.FindPython()
	if pythonBin == "" {
		return fmt.Errorf("Python not found")
	}

	// Map format name to stable-diffusion.cpp quantization type
	sdFormat := format
	switch strings.ToUpper(format) {
	case "Q4_0", "Q4_K_M", "Q4_K_S":
		sdFormat = "q4_0"
	case "Q8_0":
		sdFormat = "q8_0"
	case "Q5_0", "Q5_K_M":
		sdFormat = "q5_0"
	case "F16", "FLOAT16":
		sdFormat = "f16"
	case "F32", "FLOAT32":
		sdFormat = "f32"
	}

	script := fmt.Sprintf(`import sys, subprocess, shutil, os
model_path  = r'''%s'''
output_path = r'''%s'''
sd_format   = %q

# Try sd-convert from stable-diffusion.cpp
sd_convert = shutil.which("sd") or shutil.which("sd.exe")
if sd_convert:
    print(f"stable-diffusion.cpp found: {sd_convert}")
    cmd = [sd_convert, "--model", model_path, "--output", output_path,
           "--type", sd_format]
    print(f"Running: {' '.join(cmd)}")
    r = subprocess.run(cmd)
    if r.returncode == 0:
        print(f"\n✅  Model quantized: {output_path}")
        sys.exit(0)
    else:
        print(f"❌  Conversion failed (exit {r.returncode})")
        sys.exit(1)

# Try via stable-diffusion-cpp-python
try:
    from stable_diffusion_cpp import StableDiffusion
    print("Converting via stable-diffusion-cpp-python...")
    # stable-diffusion-cpp-python doesn't expose convert/quantize directly
    # It loads models at runtime but doesn't save
    raise NotImplementedError("stable-diffusion-cpp-python does not support conversion")
except (ImportError, NotImplementedError) as e:
    pass

print("❌  No conversion tool found.")
print()
print("Options to quantize image models:")
print()
print("1. stable-diffusion.cpp (recommended):")
print("   https://github.com/leejet/stable-diffusion.cpp/releases")
print("   sd --model", model_path, "--output", output_path, "--type", sd_format)
print()
print("2. Use pre-quantized GGUF models directly from HuggingFace:")
print("   vortelio pull image/https://huggingface.co/second-state/stable-diffusion-v1-5-GGUF?show_file_info=stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf")
sys.exit(1)
`, modelPath, outFile, sdFormat)

	tmp, _ := os.CreateTemp("", "vortelio-q-*.py")
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	cmd := exec.Command(pythonBin, tmp.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *QuantizeCommand) findLlamaDir() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".vortelio", "bin", "llama.cpp"),
		filepath.Join(home, ".vortelio", "bin"),
		"/usr/local/bin",
		"/usr/bin",
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			`C:\llama.cpp`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), "vortelio", "bin"),
		)
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "convert_hf_to_gguf.py")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "convert.py")); err == nil {
			return dir
		}
	}
	return ""
}

func (c *QuantizeCommand) findLlamaQuantize() string {
	names := []string{"llama-quantize", "llama-quantize.exe"}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{
		filepath.Join(home, ".vortelio", "bin"),
		`C:\llama.cpp\build\bin`,
		`C:\llama.cpp\bin`,
	} {
		for _, n := range names {
			p := filepath.Join(dir, n)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func (c *QuantizeCommand) printUsage() {
	fmt.Println("Quantize a model to a smaller format")
	fmt.Println()
	fmt.Println("Usage:  vortelio quantize <model> [--format Q4_K_M] [--output file.gguf]")
	fmt.Println()
	fmt.Println("Available formats:")
	fmt.Println("  LLM (GGUF):    Q2_K  Q3_K_M  Q4_K_S  Q4_K_M  Q5_K_M  Q6_K  Q8_0  F16")
	fmt.Println("  Image (GGUF):  q4_0  q5_0  q8_0  f16  f32")
	fmt.Println()
	fmt.Println("  Q4_K_M = optimal default (good quality, small file)")
	fmt.Println("  Q8_0   = high quality, larger file")
	fmt.Println("  Q2_K   = minimum file, reduced quality")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  vortelio quantize llm/mistral:7b")
	fmt.Println("  vortelio quantize llm/mistral:7b --format Q2_K")
	fmt.Println("  vortelio quantize llm/mistral:7b --format Q8_0 --output ~/mistral-q8.gguf")
	fmt.Println("  vortelio quantize image/sdxl --format q4_0")
	fmt.Println()
	fmt.Println("  ⚠️  Requires model in F16/F32 format as source.")
	fmt.Println("      Cannot re-quantize from Q3/Q4/Q5/Q8 (already compressed).")
	fmt.Println("      First download the F16 version: vortelio pull llm/hf.co/owner/repo:F16.gguf")
	fmt.Println()
	fmt.Println("      For LLM models: requires llama.cpp (vortelio setup)")
	fmt.Println("      For image models: requires stable-diffusion.cpp")
}
