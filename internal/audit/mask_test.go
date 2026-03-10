package audit

import "testing"

func TestMaskSecrets(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Sensitive flags with space separator
		{
			name: "token flag space",
			in:   "gh auth login --token ghp_abc123xyz",
			want: "gh auth login --token ****",
		},
		{
			name: "password flag space",
			in:   "mysql --password s3cret --host db",
			want: "mysql --password **** --host db",
		},
		{
			name: "api-key flag equals",
			in:   "curl --api-key=sk-abc123",
			want: "curl --api-key=****",
		},
		{
			name: "secret flag",
			in:   "vault write --secret mySecretValue",
			want: "vault write --secret ****",
		},
		{
			name: "auth flag",
			in:   "curl --auth user:pass",
			want: "curl --auth ****",
		},
		{
			name: "credentials flag",
			in:   "tool --credentials /path/to/creds.json",
			want: "tool --credentials ****",
		},
		{
			name: "client-secret flag",
			in:   "oauth --client-secret abc123",
			want: "oauth --client-secret ****",
		},

		// Bearer / Basic auth
		{
			name: "bearer token",
			in:   "curl -H Authorization: Bearer eyJhbGciOiJ...",
			want: "curl -H Authorization: Bearer ****",
		},
		{
			name: "basic auth",
			in:   "curl -H Authorization: Basic dXNlcjpwYXNz",
			want: "curl -H Authorization: Basic ****",
		},

		// Env assignments — masked by default
		{
			name: "password env",
			in:   "PASSWORD=hunter2 ./app",
			want: "PASSWORD=**** ./app",
		},
		{
			name: "api key env",
			in:   "API_KEY=sk-123abc ./deploy",
			want: "API_KEY=**** ./deploy",
		},
		{
			name: "database url env",
			in:   "DATABASE_URL=postgres://user:pass@host/db ./app",
			want: "DATABASE_URL=**** ./app",
		},

		// Env assignments — safe (not masked)
		{
			name: "TERM safe",
			in:   "TERM=xterm-256color bash",
			want: "TERM=xterm-256color bash",
		},
		{
			name: "PATH safe",
			in:   "PATH=/usr/bin:/bin ls",
			want: "PATH=/usr/bin:/bin ls",
		},
		{
			name: "LC_ prefix safe",
			in:   "LC_ALL=C sort file",
			want: "LC_ALL=C sort file",
		},
		{
			name: "XDG_ prefix safe",
			in:   "XDG_RUNTIME_DIR=/run/user/1000 app",
			want: "XDG_RUNTIME_DIR=/run/user/1000 app",
		},
		{
			name: "GIT_TERMINAL_PROMPT safe",
			in:   "GIT_TERMINAL_PROMPT=0 git fetch",
			want: "GIT_TERMINAL_PROMPT=0 git fetch",
		},
		{
			name: "PAGER safe",
			in:   "PAGER=cat git log",
			want: "PAGER=cat git log",
		},

		// No match — no change
		{
			name: "no secrets",
			in:   "kubectl get pods -A",
			want: "kubectl get pods -A",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},

		// Mixed
		{
			name: "mixed sensitive and safe",
			in:   "TERM=xterm SECRET_KEY=abc123 --token ghp_xxx cmd",
			want: "TERM=xterm SECRET_KEY=**** --token **** cmd",
		},

		// Case insensitivity for flags
		{
			name: "TOKEN uppercase flag",
			in:   "tool --TOKEN myval",
			want: "tool --TOKEN ****",
		},
		{
			name: "Password mixed case",
			in:   "tool --Password secret",
			want: "tool --Password ****",
		},

		// Multiple secrets in one command
		{
			name: "multiple sensitive flags",
			in:   "tool --token abc --password def --api-key ghi",
			want: "tool --token **** --password **** --api-key ****",
		},

		// Flag at end without value (no trailing space/value)
		{
			name: "flag at end no value",
			in:   "tool --token",
			want: "tool --token",
		},

		// Short flag variants
		{
			name: "short -token flag",
			in:   "tool -token myval",
			want: "tool -token ****",
		},
		{
			name: "short -password flag equals",
			in:   "tool -password=secret",
			want: "tool -password=****",
		},

		// Env var edge cases
		{
			name: "env var with only prefix match LC_ is safe",
			in:   "LC_CTYPE=UTF-8 cmd",
			want: "LC_CTYPE=UTF-8 cmd",
		},
		{
			name: "env var XDG_CONFIG_HOME safe",
			in:   "XDG_CONFIG_HOME=/home/user/.config cmd",
			want: "XDG_CONFIG_HOME=/home/user/.config cmd",
		},
		{
			name: "env var AGENT_CALLABLE_CONFIG_DIR safe",
			in:   "AGENT_CALLABLE_CONFIG_DIR=/tmp/test cmd",
			want: "AGENT_CALLABLE_CONFIG_DIR=/tmp/test cmd",
		},
		{
			name: "env var GH_PROMPT_DISABLED safe",
			in:   "GH_PROMPT_DISABLED=1 gh pr list",
			want: "GH_PROMPT_DISABLED=1 gh pr list",
		},
		{
			name: "unknown env var masked",
			in:   "MY_CUSTOM_VAR=something cmd",
			want: "MY_CUSTOM_VAR=**** cmd",
		},
		{
			name: "multiple env vars mixed",
			in:   "HOME=/home/user SECRET=abc TOKEN=xyz LANG=en_US.UTF-8",
			want: "HOME=/home/user SECRET=**** TOKEN=**** LANG=en_US.UTF-8",
		},

		// Bearer edge cases
		{
			name: "bearer case insensitive",
			in:   "bearer abc123",
			want: "bearer ****",
		},

		// No false positive on subcommands
		{
			name: "subcommand not masked",
			in:   "kubectl get secrets -n default",
			want: "kubectl get secrets -n default",
		},
		{
			name: "tool name not masked",
			in:   "password-manager list",
			want: "password-manager list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSecrets(tt.in)
			if got != tt.want {
				t.Errorf("maskSecrets(%q)\n  got:  %q\n  want: %q", tt.in, got, tt.want)
			}
		})
	}
}
