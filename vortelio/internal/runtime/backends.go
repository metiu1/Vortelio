package runtime

// This file wires the backend runners (llm, image, audio, video) directly into
// the runtime package so they share the same package namespace and the
// NewRunner dispatcher can reference them without import cycles.
//
// The actual runner structs are defined in:
//   internal/backends/llm/runner.go   → LLMRunner
//   internal/backends/image/runner.go → ImageRunner
//   internal/backends/audio/runner.go → AudioRunner
//   internal/backends/video/runner.go → VideoRunner
//
// Because Go does not allow cyclic imports, we keep them all in the same
// "runtime" package by placing the files under internal/backends/*/runner.go
// but declaring `package runtime` at the top of each file.
