//go:build integration

package plugin

import (
	"cloud.google.com/go/firestore"
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestQueryData(t *testing.T) {
	ds := Datasource{}

	var settings FirestoreSettings
	settings.ProjectId = "test"
	jsonSettings, err := json.Marshal(settings)
	if err != nil {
		t.Error(err)
	}

	var queries = make([]backend.DataQuery, len(queryTests))
	var byRefs = make(map[string]TestExpect, len(queryTests))
	for idx, queryTest := range queryTests {
		refID := fmt.Sprintf("ref%d", idx)
		queries[idx] = backend.DataQuery{
			RefID: refID,
			JSON:  []byte(fmt.Sprintf(`{"query": "%s"}`, queryTest.query)),
		}
		byRefs[refID] = queryTest
	}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
					JSONData: jsonSettings,
				},
			},
			Queries: queries,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Responses)
	require.Len(t, resp.Responses, len(queryTests))

	for refId, response := range resp.Responses {
		require.NoError(t, response.Error)
		testExp := byRefs[refId]
		require.Len(t, response.Frames, 1)
		require.Len(t, response.Frames[0].Fields, testExp.columnsLength)
		for _, field := range response.Frames[0].Fields {
			require.Equal(t, testExp.rowsLength, field.Len())
		}
	}
}

type healthTest struct {
	settings  string
	decrypted map[string]string
	status    backend.HealthStatus
}

var healthTests = []healthTest{
	{``, nil, backend.HealthStatusError},
	{`{}`, nil, backend.HealthStatusError},
	{`{"ProjectId": "test"}`, map[string]string{"serviceAccount": "test"}, backend.HealthStatusError},
	{`{"ProjectId": "test"}`, map[string]string{"serviceAccount": `{}`}, backend.HealthStatusError},
	{`{"ProjectId": "test"}`, nil, backend.HealthStatusOk},
}

func TestCheckHealth(t *testing.T) {
	ds := Datasource{}
	for _, test := range healthTests {
		healthResponse, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
					JSONData:                []byte(test.settings),
					DecryptedSecureJSONData: test.decrypted,
				},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, healthResponse)
		require.Equal(t, test.status, healthResponse.Status)
	}

}

type TestExpect struct {
	query         string
	rowsLength    int
	columnsLength int
	columns       []string
	frames        [][]interface{}
}

const FirestoreEmulatorHost = "FIRESTORE_EMULATOR_HOST"

var queryTests = []TestExpect{
	{
		query:         "select * from users",
		rowsLength:    5,
		columnsLength: 6,
	},
}

func newFirestoreTestClient(ctx context.Context) *firestore.Client {
	client, err := firestore.NewClient(ctx, "test")
	if err != nil {
		log.Fatalf("firebase.NewClient err: %v", err)
	}

	return client
}

func TestMain(m *testing.M) {
	// allow skipping if emulator component is unavailable
	if os.Getenv(FirestoreEmulatorHost) == "" {
		cmd := exec.Command("gcloud", "beta", "emulators", "firestore", "start", "--host-port=localhost:8765")

		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}
		defer stderr.Close()

		if err := cmd.Start(); err != nil {
			log.Println("skipping emulator-based tests: could not start emulator:", err)
			os.Exit(m.Run())
		}

		var result int
		defer func() {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			os.Exit(result)
		}()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			buf := make([]byte, 256, 256)
			for {
				n, err := stderr.Read(buf[:])
				if err != nil {
					if err == io.EOF {
						break
					}
					log.Fatalf("reading stderr %v", err)
				}
				if n > 0 {
					d := string(buf[:n])
					log.Printf("%s", d)
					if strings.Contains(d, "Dev App Server is now running") {
						wg.Done()
					}
				}
			}
		}()
		wg.Wait()

		os.Setenv(FirestoreEmulatorHost, "localhost:8765")
		ctx := context.Background()
		users := newFirestoreTestClient(ctx).Collection("users")

		usersDataRaw, _ := os.ReadFile("../../test/data/users.json")
		var usersData []map[string]interface{}
		json.Unmarshal(usersDataRaw, &usersData)
		for _, user := range usersData {
			users.Doc(fmt.Sprintf("%v", user["id"].(float64))).Set(ctx, user)
		}

		result := m.Run()
		_ = result
		return
	}

	os.Exit(m.Run())
}


