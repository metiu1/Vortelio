package runtime

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/vortelio/vortelio/internal/hub"
)

type HardwareBackend int

const (
	BackendCPU HardwareBackend = iota
	BackendCUDA
	BackendMetal
	BackendROCm
)

type Hardware struct {
	Backend    HardwareBackend
	DeviceName string
	VRAM       int64
	GPUIndex   int
}

func (h *Hardware) String() string {
	switch h.Backend {
	case BackendCUDA:
		return fmt.Sprintf("CUDA (GPU %d: %s, %.0f GB VRAM)", h.GPUIndex, h.DeviceName, float64(h.VRAM)/1e9)
	case BackendMetal:
		return fmt.Sprintf("Metal (Apple Silicon: %s)", h.DeviceName)
	case BackendROCm:
		return fmt.Sprintf("ROCm (AMD GPU: %s)", h.DeviceName)
	default:
		return "CPU"
	}
}

func DetectHardware() *Hardware {
	hw := &Hardware{Backend: BackendCPU, DeviceName: "CPU"}
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
		if err == nil && strings.Contains(string(out), "Apple") {
			hw.Backend = BackendMetal
			hw.DeviceName = strings.TrimSpace(string(out))
			return hw
		}
	}
	if nvidiaSMI, err := exec.LookPath("nvidia-smi"); err == nil {
		out, err := exec.Command(nvidiaSMI, "--query-gpu=name,memory.total", "--format=csv,noheader,nounits").Output()
		if err == nil {
			line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
			parts := strings.Split(line, ", ")
			if len(parts) >= 2 {
				hw.Backend = BackendCUDA
				hw.DeviceName = parts[0]
				var mb int64
				fmt.Sscanf(parts[1], "%d", &mb)
				hw.VRAM = mb * 1024 * 1024
				return hw
			}
		}
	}
	if _, err := exec.LookPath("rocm-smi"); err == nil {
		hw.Backend = BackendROCm
		hw.DeviceName = "AMD GPU"
		return hw
	}
	return hw
}

type RunOptions struct {
	Prompt      string
	InputFile   string
	OutputFile  string
	Steps       int
	GPU         int
	ForceCPU    bool
	Stream      bool
	ContextSize int // max context tokens for LLM (0 = use default)
}

type Runner interface {
	Run(opts *RunOptions) error
}

func NewRunner(model *hub.Model, hw *Hardware) (Runner, error) {
	switch model.Type {
	case "llm":
		return NewLLMRunner(model, hw), nil
	case "image":
		return NewImageRunner(model, hw), nil
	case "audio":
		return NewAudioRunner(model, hw), nil
	case "video":
		return NewVideoRunner(model, hw), nil
	case "3d":
		return NewThreeDRunner(model, hw), nil
	default:
		return nil, fmt.Errorf("unsupported model type: %s", model.Type)
	}
}
