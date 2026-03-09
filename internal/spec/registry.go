package spec

import "strings"

type Registry struct {
	tools map[string]ToolSpec
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]ToolSpec{}}
}

func (r *Registry) Register(t ToolSpec) {
	if r.tools == nil {
		r.tools = map[string]ToolSpec{}
	}
	r.tools[strings.ToLower(strings.TrimSpace(t.Name()))] = t
}

func (r *Registry) Get(name string) (ToolSpec, bool) {
	if r.tools == nil {
		return nil, false
	}
	t, ok := r.tools[strings.ToLower(strings.TrimSpace(name))]
	return t, ok
}

func (r *Registry) Unregister(name string) {
	delete(r.tools, strings.ToLower(strings.TrimSpace(name)))
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.tools))
	for k := range r.tools {
		out = append(out, k)
	}
	return out
}
