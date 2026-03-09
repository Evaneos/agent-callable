package gcloud

import (
	"testing"

	"github.com/evaneos/agent-callable/internal/spectest"
)

func TestGcloudAllowlist(t *testing.T) {
	tool := New()
	allowed := [][]string{
		{"version"},
		{"info"},
		{"config", "list"},
		{"auth", "list"},
		{"projects", "list"},
		{"projects", "describe", "myproj"},
		{"compute", "instances", "list"},
		{"container", "clusters", "describe", "c1"},
		{"logging", "read", "resource.type=gce_instance"},
		// Global flags with value argument
		{"--project", "my-project", "compute", "instances", "list"},
		{"--project", "my-project", "config", "list"},
		{"--format", "json", "projects", "list"},
		{"--verbosity", "debug", "info"},
		{"--project", "p1", "--format", "json", "container", "clusters", "list"},
		// config read
		{"config", "get-value", "project"},
		{"config", "get", "project"},
		// Deep commands (4+ levels)
		{"iam", "service-accounts", "keys", "list", "--iam-account=sa@proj.iam.gserviceaccount.com"},
		{"compute", "networks", "subnets", "list"},
		{"compute", "networks", "subnets", "describe", "my-subnet"},
		{"compute", "routers", "nats", "list"},
		{"compute", "routers", "nats", "describe", "my-nat"},
		// get-credentials (local kubeconfig write)
		{"container", "clusters", "get-credentials", "my-cluster"},
		{"--project", "my-proj", "container", "clusters", "get-credentials", "c1", "--zone", "europe-west1-b"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestGcloudBlocksWrites(t *testing.T) {
	tool := New()
	blocked := [][]string{
		{"projects", "create", "x"},
		{"services", "enable", "x"},
		{"compute", "instances", "delete", "x"},
		{"auth", "login"},
		{"config", "set", "project", "x"},
		// Global flags don't bypass sub-command checks
		{"--project", "my-project", "compute", "instances", "delete", "x"},
		{"--project", "my-project", "services", "enable", "x"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestGcloudEdgeCases(t *testing.T) {
	tool := New()

	// === ALLOWED edge cases ===
	allowed := [][]string{
		// Various deep read-only commands
		{"sql", "instances", "list"},
		{"sql", "databases", "list", "--instance=myinst"},
		{"dns", "record-sets", "list", "--zone=myzone"},
		{"pubsub", "topics", "list"},
		{"pubsub", "subscriptions", "list"},
		{"storage", "buckets", "list"},
		{"functions", "list"},
		{"functions", "describe", "myfunc"},
		{"app", "instances", "list"},
		{"app", "services", "list"},
		{"redis", "instances", "describe", "myinst"},
		{"redis", "instances", "list"},
		// Networking
		{"compute", "addresses", "list"},
		{"compute", "firewall-rules", "list"},
		{"compute", "forwarding-rules", "list"},
		{"compute", "target-pools", "list"},
		// IAM read-only
		{"iam", "roles", "list"},
		{"iam", "roles", "describe", "roles/editor"},
		{"iam", "service-accounts", "list"},
		{"iam", "service-accounts", "describe", "sa@proj.iam.gserviceaccount.com"},
		// Deeply nested describe
		{"compute", "networks", "peerings", "list", "--network=default"},
		// Multiple global flags
		{"--project", "p1", "--format", "json", "--verbosity", "info", "compute", "instances", "list"},
		// config read variants
		{"config", "list", "--all"},
		// auth list
		{"auth", "list"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	// === BLOCKED edge cases ===
	blocked := [][]string{
		// New write verbs
		{"compute", "instances", "reset", "myinst"},
		{"compute", "disks", "move", "mydisk"},
		{"compute", "firewall-rules", "insert", "myrule"},
		{"sql", "instances", "import", "myinst"},
		{"sql", "instances", "export", "myinst"},
		{"sql", "instances", "patch", "myinst"},
		{"compute", "instances", "suspend", "myinst"},
		{"compute", "instances", "resume", "myinst"},
		{"compute", "addresses", "remove", "myaddr"},
		{"compute", "instances", "resize", "myinst"},
		// Deployment commands
		{"app", "deploy"},
		{"functions", "deploy", "myfunc"},
		{"run", "deploy", "myservice"},
		// Start/stop
		{"compute", "instances", "start", "myinst"},
		{"compute", "instances", "stop", "myinst"},
		{"sql", "instances", "restart", "myinst"},
		// IAM write
		{"iam", "service-accounts", "create", "sa@proj.iam.gserviceaccount.com"},
		{"iam", "service-accounts", "delete", "sa@proj.iam.gserviceaccount.com"},
		{"projects", "add-iam-policy-binding", "myproj"},
		{"projects", "remove-iam-policy-binding", "myproj"},
		// Storage write
		{"storage", "buckets", "create", "mybucket"},
		{"storage", "buckets", "delete", "mybucket"},
		// Config write
		{"config", "set", "project", "myproj"},
		{"config", "unset", "project"},
		// Auth login
		{"auth", "login"},
		{"auth", "activate-service-account"},
		{"auth", "revoke"},
		// Global flags don't bypass
		{"--project", "my-proj", "compute", "instances", "delete", "x"},
		{"--format", "json", "services", "enable", "compute.googleapis.com"},
		// No read-only verb anywhere
		{"compute", "instances"},
		{"sql", "databases"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestGcloudEmptyAndBareArgs(t *testing.T) {
	tool := New()
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"--project", "myproj"})
}
