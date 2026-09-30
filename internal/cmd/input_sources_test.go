package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestInputSourcesBuildTheRequestBody(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			name: "an inline payload is sent verbatim",
			args: []string{"components", "create", "--payload", `{"name":"api","version":"1.2.3"}`},
			want: map[string]any{"name": "api", "version": "1.2.3"},
		},
		{
			name: "a yaml file is decoded before it is sent",
			args: []string{"components", "create", "--file", "PLACEHOLDER"},
			want: map[string]any{"name": "api", "version": "1.2.3"},
		},
		{
			name: "inline flags are assembled into a payload",
			args: []string{"components", "create", "--name", "api", "--version", "1.2.3"},
			want: map[string]any{"name": "api", "version": "1.2.3"},
		},
		{
			name: "an optional field the caller set is included",
			args: []string{"components", "create", "--name", "api", "--version", "1.2.3", "--slug", "api-slug"},
			want: map[string]any{"name": "api", "version": "1.2.3", "slug": "api-slug"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			givenIsolatedConfig(t)
			args := test.args
			if args[len(args)-1] == "PLACEHOLDER" {
				args = append(args[:len(args)-1], givenInputFile(t, "component.yaml", "name: api\nversion: 1.2.3\n"))
			}
			recorder := &requestRecorder{}
			server := givenComponentAPI(t, recorder)

			runSuccessfully(t, append([]string{"--base-url", server.URL}, args...)...)

			expectRequestBody(t, recorder, test.want)
		})
	}
}

// TestOptionalFieldsAreOmittedWhenTheirFlagIsUnset pins the rule optionalString exists for:
// a field the caller never mentioned must not reach the API at all, because an empty string
// would overwrite whatever the record already holds.
func TestOptionalFieldsAreOmittedWhenTheirFlagIsUnset(t *testing.T) {
	givenIsolatedConfig(t)
	recorder := &requestRecorder{}
	server := givenComponentAPI(t, recorder)

	runSuccessfully(t, "--base-url", server.URL, "components", "create", "--name", "api", "--version", "1.2.3")

	body := decodeRecordedBody(t, recorder)
	for _, absent := range []string{"slug", "description", "previousComponentId"} {
		if _, ok := body[absent]; ok {
			t.Errorf("request body carries %q = %v, want it omitted", absent, body[absent])
		}
	}
}

// TestExplicitEmptyOptionalFieldIsSent is the other half of the rule above: --slug "" is a
// deliberate instruction to clear the field, and has to be told apart from an unset flag.
func TestExplicitEmptyOptionalFieldIsSent(t *testing.T) {
	givenIsolatedConfig(t)
	recorder := &requestRecorder{}
	server := givenComponentAPI(t, recorder)

	runSuccessfully(t, "--base-url", server.URL, "components", "create", "--name", "api", "--version", "1.2.3", "--slug", "")

	body := decodeRecordedBody(t, recorder)
	if got, ok := body["slug"]; !ok || got != "" {
		t.Fatalf("request body slug = %v (present: %t), want an explicit empty string", got, ok)
	}
}

func TestInlineFlagsRejectAMissingRequiredField(t *testing.T) {
	givenIsolatedConfig(t)

	// --version is required alongside --name, and no request should leave the CLI.
	recorder := &requestRecorder{}
	server := givenComponentAPI(t, recorder)

	_, err := executeForExitCode(t, "--base-url", server.URL, "components", "create", "--name", "api")

	if err == nil || !strings.Contains(err.Error(), "--version") {
		t.Fatalf("Execute() error = %v, want it to name --version", err)
	}
	if calls, _, _, _ := recorder.snapshot(); calls != 0 {
		t.Fatalf("the API was called %d times, want the CLI to stop before the request", calls)
	}
}

func TestFileInputRejectsAMalformedDocument(t *testing.T) {
	givenIsolatedConfig(t)
	path := givenInputFile(t, "component.json", "{not json")

	_, err := executeForExitCode(t, "--base-url", "http://127.0.0.1:1", "components", "create", "--file", path)

	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("Execute() error = %v, want a decode failure", err)
	}
}

func TestFileInputReportsAMissingFile(t *testing.T) {
	givenIsolatedConfig(t)

	_, err := executeForExitCode(t, "--base-url", "http://127.0.0.1:1", "components", "create", "--file", t.TempDir()+"/absent.yaml")

	if err == nil || !strings.Contains(err.Error(), "read input file") {
		t.Fatalf("Execute() error = %v, want a read failure", err)
	}
}

// requestRecorder captures what the CLI sent so the assertions can run on the test's own
// goroutine. Asserting inside the handler would call t.Fatalf from the server's goroutine,
// where it stops that goroutine instead of failing the test. The mutex guards the handoff:
// the handler runs on the server's goroutine, every reader is the test's.
type requestRecorder struct {
	mu            sync.Mutex
	calls         int
	method        string
	path          string
	body          []byte
	authorization string
}

func (r *requestRecorder) record(req *http.Request, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls++
	r.method = req.Method
	r.path = req.URL.Path
	r.body = body
}

func (r *requestRecorder) recordHeader(value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.authorization = value
}

func (r *requestRecorder) header() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.authorization
}

func (r *requestRecorder) snapshot() (calls int, method, path string, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.calls, r.method, r.path, r.body
}

func givenComponentAPI(t *testing.T, recorder *requestRecorder) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recorder.record(r, body)

		writeTestJSON(w, http.StatusCreated, `{"id":"cmp_1","name":"api","slug":"api","version":"1.2.3","createdBy":{}}`)
	}))
	t.Cleanup(server.Close)

	return server
}

func givenInputFile(t *testing.T, name, content string) string {
	t.Helper()

	path := t.TempDir() + "/" + name
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}

func decodeRecordedBody(t *testing.T, recorder *requestRecorder) map[string]any {
	t.Helper()

	calls, _, _, raw := recorder.snapshot()
	if calls != 1 {
		t.Fatalf("the API was called %d times, want exactly 1", calls)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode request body error = %v; body = %q", err, raw)
	}

	return body
}

func expectRequestBody(t *testing.T, recorder *requestRecorder, want map[string]any) {
	t.Helper()

	_, method, path, _ := recorder.snapshot()
	if method != http.MethodPost || path != "/components" {
		t.Fatalf("request = %s %s, want POST /components", method, path)
	}
	body := decodeRecordedBody(t, recorder)
	for key, wantValue := range want {
		if got := body[key]; got != wantValue {
			t.Errorf("request body %s = %v, want %v", key, got, wantValue)
		}
	}
}

// TestInlineFlagsAreRecognisedByTheCommandThatRegistersThem is the guard that lets each
// resource keep its flag names as plain literals rather than 20 one-line constants.
//
// A name is written three times per field: once to register the flag, once in the
// "did the caller use inline flags?" check, and once to read it back. A typo in the
// second or third copy does not fail to compile -- it makes the flag silently invisible,
// so the CLI reports "provide one input source" for an invocation that plainly used one,
// or drops a field the caller set. This walks every command that accepts inline flags and
// invokes it with each of its own flags alone, asserting the command recognises it.
func TestInlineFlagsAreRecognisedByTheCommandThatRegistersThem(t *testing.T) {
	givenIsolatedConfig(t)

	// Flags that are not part of an inline payload, and so are not expected to select it.
	notPayloadFlags := map[string]bool{"file": true, "payload": true, "yes": true, "help": true}

	walkCommands(NewRootCommand(), func(cmd *cobra.Command) {
		if !strings.Contains(cmd.Short, "inline flags") {
			return
		}

		cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
			if notPayloadFlags[flag.Name] {
				return
			}

			t.Run(cmd.CommandPath()+" --"+flag.Name, func(t *testing.T) {
				givenIsolatedConfig(t)

				args := append(strings.Split(cmd.CommandPath(), " ")[1:], placeholderArgs(cmd)...)
				args = append([]string{"--base-url", "http://127.0.0.1:1"}, args...)
				args = append(args, "--"+flag.Name, placeholderValue(flag))

				_, err := executeForExitCode(t, args...)

				// The command is pointed at a dead port, so it must fail -- but on the
				// request, or on a missing sibling field, never on "no input source".
				if err != nil && strings.Contains(err.Error(), "provide one input source") {
					t.Fatalf("--%s did not register as inline input: %v", flag.Name, err)
				}
			})
		})
	})
}

// placeholderArgs supplies one throwaway value per positional argument in the command's
// Use line, so that argument validation is satisfied and the run reaches the input check.
func placeholderArgs(cmd *cobra.Command) []string {
	var args []string
	for _, field := range strings.Fields(cmd.Use)[1:] {
		if strings.HasPrefix(field, "<") && strings.HasSuffix(field, ">") {
			args = append(args, "placeholder")
		}
	}
	return args
}

func placeholderValue(flag *pflag.Flag) string {
	switch flag.Value.Type() {
	case "bool":
		return "true"
	case "float64", "int32":
		return "1"
	default:
		return "placeholder"
	}
}
