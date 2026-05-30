package server

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/vortelio/vortelio/internal/hub"
)

// ─────────────────────────────────────────────────────────────────────────────
// /api/gguf/inspect — parse GGUF header
//
// GGUF v3 format: https://github.com/ggerganov/ggml/blob/master/docs/gguf.md
// Layout:
//   magic        u32  ("GGUF")
//   version      u32
//   tensor_count u64
//   metadata_kv_count u64
//   metadata_kv[]    (key:string, type:u32, value)
//   tensor_info[]    (name:string, n_dims:u32, dims[n_dims]:u64, type:u32, offset:u64)
// ─────────────────────────────────────────────────────────────────────────────

const (
	ggufTypeUInt8   = 0
	ggufTypeInt8    = 1
	ggufTypeUInt16  = 2
	ggufTypeInt16   = 3
	ggufTypeUInt32  = 4
	ggufTypeInt32   = 5
	ggufTypeFloat32 = 6
	ggufTypeBool    = 7
	ggufTypeString  = 8
	ggufTypeArray   = 9
	ggufTypeUInt64  = 10
	ggufTypeInt64   = 11
	ggufTypeFloat64 = 12
)

func handleGGUFInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		Path  string `json:"path"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}

	path := req.Path
	if path == "" && req.Model != "" {
		ref, err := hub.ParseModelRef(req.Model)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		m, err := hub.NewModelStore().Resolve(ref)
		if err != nil {
			jsonError(w, 404, err.Error())
			return
		}
		path = m.LocalPath
	}
	if path == "" {
		jsonError(w, 400, "path or model required")
		return
	}

	f, err := os.Open(path)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	defer f.Close()

	info, err := parseGGUF(f)
	if err != nil {
		jsonError(w, 500, "GGUF parse error: "+err.Error())
		return
	}
	respond(w, 200, info)
}

func parseGGUF(f *os.File) (map[string]interface{}, error) {
	g := &ggufReader{r: f}
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, err
	}
	if string(magic) != "GGUF" {
		return nil, fmt.Errorf("not a GGUF file (magic=%q)", magic)
	}
	version := g.u32()
	tensorCount := g.u64()
	kvCount := g.u64()
	if g.err != nil {
		return nil, g.err
	}

	metadata := make(map[string]interface{})
	for i := uint64(0); i < kvCount; i++ {
		key := g.str()
		val := g.value()
		if g.err != nil {
			break
		}
		metadata[key] = val
		if i > 5000 {
			break
		}
	}

	// Tensor info — name + dims + type only (skip data)
	var tensors []map[string]interface{}
	tensorCap := tensorCount
	if tensorCap > 2000 {
		tensorCap = 2000
	}
	for i := uint64(0); i < tensorCap && g.err == nil; i++ {
		name := g.str()
		nDims := g.u32()
		dims := make([]uint64, nDims)
		for j := uint32(0); j < nDims; j++ {
			dims[j] = g.u64()
		}
		typ := g.u32()
		_ = g.u64() // offset
		tensors = append(tensors, map[string]interface{}{
			"name": name,
			"dims": dims,
			"type": typ,
		})
	}

	st, _ := f.Stat()
	return map[string]interface{}{
		"path":           f.Name(),
		"file_size":      st.Size(),
		"gguf_version":   version,
		"tensor_count":   tensorCount,
		"metadata_count": kvCount,
		"metadata":       metadata,
		"tensor_sample":  tensors,
	}, nil
}

type ggufReader struct {
	r   io.Reader
	err error
}

func (g *ggufReader) u32() uint32 {
	if g.err != nil {
		return 0
	}
	var v uint32
	g.err = binary.Read(g.r, binary.LittleEndian, &v)
	return v
}

func (g *ggufReader) u64() uint64 {
	if g.err != nil {
		return 0
	}
	var v uint64
	g.err = binary.Read(g.r, binary.LittleEndian, &v)
	return v
}

func (g *ggufReader) i32() int32 {
	if g.err != nil {
		return 0
	}
	var v int32
	g.err = binary.Read(g.r, binary.LittleEndian, &v)
	return v
}

func (g *ggufReader) i64() int64 {
	if g.err != nil {
		return 0
	}
	var v int64
	g.err = binary.Read(g.r, binary.LittleEndian, &v)
	return v
}

func (g *ggufReader) f32() float32 {
	if g.err != nil {
		return 0
	}
	var v float32
	g.err = binary.Read(g.r, binary.LittleEndian, &v)
	return v
}

func (g *ggufReader) f64() float64 {
	if g.err != nil {
		return 0
	}
	var v float64
	g.err = binary.Read(g.r, binary.LittleEndian, &v)
	return v
}

func (g *ggufReader) u8() uint8 {
	if g.err != nil {
		return 0
	}
	var v uint8
	g.err = binary.Read(g.r, binary.LittleEndian, &v)
	return v
}

func (g *ggufReader) bool_() bool {
	return g.u8() != 0
}

func (g *ggufReader) str() string {
	n := g.u64()
	if g.err != nil || n > 100*1024*1024 {
		return ""
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(g.r, buf); err != nil {
		g.err = err
		return ""
	}
	return string(buf)
}

func (g *ggufReader) value() interface{} {
	t := g.u32()
	return g.valueOfType(t)
}

func (g *ggufReader) valueOfType(t uint32) interface{} {
	switch t {
	case ggufTypeUInt8:
		return g.u8()
	case ggufTypeInt8:
		var v int8
		g.err = binary.Read(g.r, binary.LittleEndian, &v)
		return v
	case ggufTypeUInt16:
		var v uint16
		g.err = binary.Read(g.r, binary.LittleEndian, &v)
		return v
	case ggufTypeInt16:
		var v int16
		g.err = binary.Read(g.r, binary.LittleEndian, &v)
		return v
	case ggufTypeUInt32:
		return g.u32()
	case ggufTypeInt32:
		return g.i32()
	case ggufTypeFloat32:
		return g.f32()
	case ggufTypeBool:
		return g.bool_()
	case ggufTypeString:
		return g.str()
	case ggufTypeArray:
		subType := g.u32()
		n := g.u64()
		if n > 100000 {
			n = 100000
		}
		out := make([]interface{}, 0, n)
		for i := uint64(0); i < n && g.err == nil; i++ {
			out = append(out, g.valueOfType(subType))
		}
		return out
	case ggufTypeUInt64:
		return g.u64()
	case ggufTypeInt64:
		return g.i64()
	case ggufTypeFloat64:
		return g.f64()
	}
	g.err = fmt.Errorf("unknown GGUF type %d", t)
	return nil
}
