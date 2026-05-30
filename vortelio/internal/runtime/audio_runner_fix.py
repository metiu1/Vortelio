with open('/home/claude/pullai-0.3.38/pullai/internal/runtime/audio_runner.go', 'r') as f:
    go = f.read()

func_start = go.find('func (r *AudioRunner) buildWhisperScript(opts *RunOptions) string {')
func_end   = go.find('\nfunc (r *AudioRunner) buildBarkScript', func_start)
assert func_start > 0 and func_end > func_start, "function boundaries not found"

new_func = r'''func (r *AudioRunner) buildWhisperScript(opts *RunOptions) string {
	inputPath := strings.ReplaceAll(opts.InputFile, `\`, `/`)
	device    := r.deviceString(opts.ForceCPU)
	tag       := r.model.Tag

	// Use string concatenation — avoids fmt.Sprintf percent-escaping issues entirely
	s := "import sys, os\n"
	s += `os.environ["PYTHONIOENCODING"] = "utf-8"` + "\n\n"
	s += "_lib = None\n"
	s += "try:\n    from faster_whisper import WhisperModel as _FW\n    _lib = 'faster-whisper'\n"
	s += "except ImportError:\n    pass\n\n"
	s += "if _lib is None:\n    try:\n        import whisper as _W\n        _lib = 'openai-whisper'\n    except Exception:\n        pass\n\n"
	s += "if _lib is None:\n    print('ERROR: pip install faster-whisper')\n    sys.exit(1)\n\n"
	s += "try:\n    import torch as _t\n    _cuda_ok = _t.cuda.is_available()\n    del _t\nexcept ImportError:\n    _cuda_ok = False\n\n"
	s += "_dev = 'cuda' if _cuda_ok and '" + device + "' != 'cpu' else 'cpu'\n\n"
	s += "_tag_map = {\n"
	s += "    'large': 'large-v3', 'large-v3': 'large-v3', 'large-v2': 'large-v2',\n"
	s += "    'large-v1': 'large-v1', 'medium': 'medium', 'small': 'small',\n"
	s += "    'base': 'base', 'tiny': 'tiny', 'turbo': 'large-v3-turbo',\n"
	s += "    'large-v3-turbo': 'large-v3-turbo', 'distil-large-v3': 'distil-large-v3',\n"
	s += "}\n"
	s += "import re as _re\n"
	s += "_base = _re.split(r'[/\\\\]', '" + tag + "')[-1].lower().strip()\n"
	s += "_fw_model = _tag_map.get(_base, _tag_map.get('" + tag + "'.lower(), 'large-v3'))\n\n"
	s += "if _lib == 'faster-whisper':\n"
	s += "    _ct = 'float16' if _dev == 'cuda' else 'int8'\n"
	s += "    print('Loading: ' + _fw_model + ' (' + _ct + ')')\n"
	s += "    _m = _FW(_fw_model, device=_dev, compute_type=_ct)\n"
	s += "    _segs, _ = _m.transcribe(r'" + inputPath + "', beam_size=5)\n"
	s += "    _text = ''.join(seg.text for seg in _segs)\n"
	s += "else:\n"
	s += "    _m = _W.load_model(_fw_model, device=_dev)\n"
	s += "    _res = _W.transcribe(_m, r'" + inputPath + "', fp16=(_dev != 'cpu'))\n"
	s += "    _text = _res['text']\n\n"
	s += "print(_text)\n"
	if opts.OutputFile != "" {
		outPath := strings.ReplaceAll(ResolveOutputPath(opts.OutputFile, ""), `\`, `/`)
		s += "\nwith open(r'" + outPath + "', 'w', encoding='utf-8') as f:\n"
		s += "    f.write(_text)\n"
		s += "print('Saved to: " + outPath + "')\n"
	}
	return s
}'''

go = go[:func_start] + new_func + '\n' + go[func_end:]

with open('/home/claude/pullai-0.3.38/pullai/internal/runtime/audio_runner.go', 'w') as f:
    f.write(go)
print("buildWhisperScript rewritten OK")
