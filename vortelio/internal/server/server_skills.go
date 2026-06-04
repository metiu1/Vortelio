package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vortelio/vortelio/internal/config"
)

// Skill is a reusable instruction set (optionally tool-aware) the user can enable
// for an agentic chat. Custom skills are markdown files with simple frontmatter
// stored under <home>/skills; a few useful skills ship built-in.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Builtin     bool   `json:"builtin"`
}

var builtinSkills = []Skill{
	{
		ID:          "web-researcher",
		Name:        "Web Researcher",
		Description: "Search the web, cross-check sources, and cite links.",
		Body:        "You are a meticulous web researcher. When asked about current events, facts, prices, or anything time-sensitive, ALWAYS use the web_search tool before answering. Cross-check at least two sources, summarize concisely, and cite the URLs you used.",
		Builtin:     true,
	},
	{
		ID:          "code-reviewer",
		Name:        "Code Reviewer",
		Description: "Review code for bugs, security and clarity.",
		Body:        "You are a senior code reviewer. Read the relevant files before commenting. Focus on correctness bugs, security issues, and clear, actionable fixes. Reference exact file paths and line context. Be concise.",
		Builtin:     true,
	},
	{
		ID:          "planner",
		Name:        "Step Planner",
		Description: "Break a task into a concrete, ordered plan before acting.",
		Body:        "Before taking any action, produce a short numbered plan of the steps you will take. Execute one step at a time and report progress. Stop and ask if a step is ambiguous or risky.",
		Builtin:     true,
	},
}

func skillsDir() string {
	return filepath.Join(config.HomeDir(), "skills")
}

var frontmatterRE = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n?`)

// parseSkillFile reads a skill markdown file with optional frontmatter.
func parseSkillFile(id, raw string) Skill {
	s := Skill{ID: id, Name: id}
	if m := frontmatterRE.FindStringSubmatch(raw); m != nil {
		for _, line := range strings.Split(m[1], "\n") {
			kv := strings.SplitN(line, ":", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			switch key {
			case "name":
				s.Name = val
			case "description":
				s.Description = val
			}
		}
		s.Body = strings.TrimSpace(raw[len(m[0]):])
	} else {
		s.Body = strings.TrimSpace(raw)
	}
	return s
}

// listSkills returns built-in skills plus any custom ones on disk.
func listSkills() []Skill {
	out := append([]Skill{}, builtinSkills...)
	entries, err := os.ReadDir(skillsDir())
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(skillsDir(), e.Name()))
			if err != nil {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".md")
			out = append(out, parseSkillFile(id, string(data)))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func skillByID(id string) (Skill, bool) {
	for _, s := range listSkills() {
		if s.ID == id {
			return s, true
		}
	}
	return Skill{}, false
}

// applySkills augments a system prompt with the bodies of the enabled skills.
func applySkills(system string, ids []string) string {
	var parts []string
	if strings.TrimSpace(system) != "" {
		parts = append(parts, system)
	}
	for _, id := range ids {
		if s, ok := skillByID(id); ok && s.Body != "" {
			parts = append(parts, "# Skill: "+s.Name+"\n"+s.Body)
		}
	}
	return strings.Join(parts, "\n\n")
}

var skillIDRE = regexp.MustCompile(`[^a-z0-9_-]+`)

// GET  /api/skills        — list skills
// POST /api/skills        — create/update a custom skill {id?, name, description, body}
func handleSkills(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		respond(w, 200, map[string]interface{}{"skills": listSkills()})
	case http.MethodPost:
		var req struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Body        string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, 400, "invalid JSON: "+err.Error())
			return
		}
		if req.Name == "" && req.ID == "" {
			jsonError(w, 400, "name is required")
			return
		}
		id := req.ID
		if id == "" {
			id = skillIDRE.ReplaceAllString(strings.ToLower(strings.ReplaceAll(req.Name, " ", "-")), "")
		}
		if id == "" {
			jsonError(w, 400, "could not derive a valid skill id")
			return
		}
		content := "---\nname: " + req.Name + "\ndescription: " + req.Description + "\n---\n" + req.Body
		if err := os.MkdirAll(skillsDir(), 0755); err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		if err := os.WriteFile(filepath.Join(skillsDir(), id+".md"), []byte(content), 0644); err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		respond(w, 200, map[string]interface{}{"status": "saved", "id": id, "skills": listSkills()})
	default:
		jsonError(w, 405, "use GET or POST")
	}
}

// POST /api/skills/delete  — {id}
func handleSkillDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "use POST")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	for _, b := range builtinSkills {
		if b.ID == req.ID {
			jsonError(w, 400, "cannot delete a built-in skill")
			return
		}
	}
	if err := os.Remove(filepath.Join(skillsDir(), req.ID+".md")); err != nil && !os.IsNotExist(err) {
		jsonError(w, 500, err.Error())
		return
	}
	respond(w, 200, map[string]interface{}{"status": "ok", "skills": listSkills()})
}
