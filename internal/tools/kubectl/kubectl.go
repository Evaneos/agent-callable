package kubectl

import (
	"github.com/Evaneos/kubectl-readonly/kubepolicy"
	"github.com/evaneos/agent-callable/internal/spec"
)

type Tool struct{}

func New() *Tool { return &Tool{} }

func (t *Tool) Name() string { return "kubectl" }

func (t *Tool) NonInteractiveEnv() map[string]string { return nil }

func (t *Tool) Check(args []string, _ spec.RuntimeCtx) spec.Result {
	allowed, hint := kubepolicy.Check(args)
	if !allowed {
		if hint != "" {
			return spec.Deny(hint)
		}
		return spec.Deny("kubectl command not allowed")
	}
	return spec.Allow()
}
