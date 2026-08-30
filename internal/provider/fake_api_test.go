package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAPI is an in-memory stand-in for the FoPost API, enough of it for the
// acceptance tests to drive a real create/read/update/delete/import cycle
// without touching the network.
type fakeAPI struct {
	t *testing.T

	mu          sync.Mutex
	nextID      int
	workspaces  map[string]map[string]any
	labels      map[string]map[string]any
	webhooks    map[string]map[string]any
	automations map[string]map[string]any
	accounts    map[string]map[string]any

	// failNext, when set, makes the next matching request answer that status.
	failNext map[string]int
}

const fakeAPIKey = "fp_test_key_never_logged"

func newFakeAPI(t *testing.T) (*fakeAPI, *httptest.Server) {
	t.Helper()
	api := &fakeAPI{
		t:           t,
		workspaces:  map[string]map[string]any{},
		labels:      map[string]map[string]any{},
		webhooks:    map[string]map[string]any{},
		automations: map[string]map[string]any{},
		accounts:    map[string]map[string]any{},
		failNext:    map[string]int{},
	}
	server := httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(server.Close)
	return api, server
}

func (a *fakeAPI) id(prefix string) string {
	a.nextID++
	return fmt.Sprintf("%s_%03d", prefix, a.nextID)
}

func (a *fakeAPI) addAccount(workspaceID, platform, username string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := a.id("acc")
	a.accounts[id] = map[string]any{
		"id":              id,
		"workspaceId":     workspaceID,
		"workspace_id":    workspaceID,
		"platform":        platform,
		"username":        username,
		"name":            strings.ToUpper(username[:1]) + username[1:],
		"avatar":          "https://cdn.example.com/" + username + ".png",
		"isPrimary":       true,
		"active":          true,
		"healthStatus":    "healthy",
		"lastHealthCheck": "2026-08-30T09:00:00Z",
		"workspace": map[string]any{
			"id": workspaceID, "name": "Fixture", "slug": "fixture", "type": "TEAM",
		},
		"created_at": "2026-08-30T09:00:00Z",
		"updated_at": "2026-08-30T09:00:00Z",
	}
	return id
}

func (a *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("X-API-Key"); got != fakeAPIKey {
		a.write(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized", "message": "bad key"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if status := a.failNext[r.Method+" "+r.URL.Path]; status != 0 {
		delete(a.failNext, r.Method+" "+r.URL.Path)
		a.write(w, status, map[string]any{"error": "forced_failure", "message": "the fixture forced this"})
		return
	}

	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1"), "/"), "/")
	switch parts[0] {
	case "workspaces":
		a.collection(w, r, parts, a.workspaces, "ws", a.newWorkspace)
	case "labels":
		a.collection(w, r, parts, a.labels, "lbl", a.newLabel)
	case "webhooks":
		a.collection(w, r, parts, a.webhooks, "whk", a.newWebhook)
	case "automations":
		a.collection(w, r, parts, a.automations, "atm", a.newAutomation)
	case "accounts":
		a.collection(w, r, parts, a.accounts, "acc", nil)
	default:
		a.write(w, http.StatusNotFound, map[string]any{"error": "not_found", "message": "no such endpoint"})
	}
}

type factory func(body map[string]any, id string) map[string]any

func (a *fakeAPI) collection(w http.ResponseWriter, r *http.Request, parts []string, store map[string]map[string]any, prefix string, build factory) {
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			list := records(store)
			a.write(w, http.StatusOK, map[string]any{"data": list})
		case http.MethodPost:
			if build == nil {
				a.write(w, http.StatusMethodNotAllowed, map[string]any{"error": "not_allowed"})
				return
			}
			record := build(a.decode(r), a.id(prefix))
			store[record["id"].(string)] = record
			a.write(w, http.StatusCreated, map[string]any{"data": record})
		default:
			a.write(w, http.StatusMethodNotAllowed, map[string]any{"error": "not_allowed"})
		}
		return
	}

	id := parts[1]
	record, found := store[id]
	if !found {
		a.write(w, http.StatusNotFound, map[string]any{"error": "not_found", "message": "no such object"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.write(w, http.StatusOK, map[string]any{"data": record})
	case http.MethodPut:
		for key, value := range a.decode(r) {
			record[camel(key)] = value
			record[key] = value
		}
		record["updated_at"] = time.Now().UTC().Format(time.RFC3339)
		record["updatedAt"] = record["updated_at"]
		normalizeSteps(record)
		delete(record, "secret")
		a.write(w, http.StatusOK, map[string]any{"data": record})
	case http.MethodDelete:
		delete(store, id)
		a.write(w, http.StatusOK, map[string]any{"data": map[string]any{"message": "deleted"}})
	default:
		a.write(w, http.StatusMethodNotAllowed, map[string]any{"error": "not_allowed"})
	}
}

func (a *fakeAPI) newWorkspace(body map[string]any, id string) map[string]any {
	record := map[string]any{
		"id":         id,
		"name":       body["name"],
		"slug":       body["slug"],
		"type":       orDefault(body["type"], "PERSONAL"),
		"logo":       orDefault(body["logo"], ""),
		"website":    orDefault(body["website"], ""),
		"timezone":   orDefault(body["timezone"], "UTC"),
		"country":    orDefault(body["country"], ""),
		"language":   orDefault(body["language"], "en"),
		"created_at": "2026-08-30T09:00:00Z",
		"updated_at": "2026-08-30T09:00:00Z",
	}
	record["description"] = orDefault(body["description"], "")
	record["accounts"] = []any{}
	return record
}

func (a *fakeAPI) newLabel(body map[string]any, id string) map[string]any {
	workspaceID, _ := body["workspace_id"].(string)
	return map[string]any{
		"id":    id,
		"name":  body["name"],
		"color": body["color"],
		"workspace": map[string]any{
			"id": workspaceID, "name": "Fixture", "slug": "fixture", "type": "TEAM",
		},
		"created_at": "2026-08-30T09:00:00Z",
		"updated_at": "2026-08-30T09:00:00Z",
	}
}

func (a *fakeAPI) newWebhook(body map[string]any, id string) map[string]any {
	return map[string]any{
		"id":           id,
		"workspaceId":  body["workspaceId"],
		"url":          body["url"],
		"events":       body["events"],
		"active":       true,
		"secret":       "whsec_fixture",
		"failureCount": 0,
		"createdAt":    "2026-08-30T09:00:00Z",
	}
}

func (a *fakeAPI) newAutomation(body map[string]any, id string) map[string]any {
	record := map[string]any{
		"id":            id,
		"workspaceId":   body["workspaceId"],
		"name":          body["name"],
		"triggerType":   body["triggerType"],
		"triggerConfig": body["triggerConfig"],
		"active":        orDefault(body["active"], true),
		"steps":         body["steps"],
		"runCount":      0,
		"createdAt":     "2026-08-30T09:00:00Z",
		"updatedAt":     "2026-08-30T09:00:00Z",
	}
	if body["triggerType"] == "api_webhook" {
		record["secret"] = "atsec_fixture"
	}
	normalizeSteps(record)
	return record
}

// normalizeSteps numbers the steps the way the API does, from their order.
func normalizeSteps(record map[string]any) {
	steps, ok := record["steps"].([]any)
	if !ok {
		return
	}
	for i, raw := range steps {
		step, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		step["position"] = i + 1
	}
}

func (a *fakeAPI) decode(r *http.Request) map[string]any {
	a.t.Helper()
	body := map[string]any{}
	if r.Body == nil {
		return body
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
		a.t.Fatalf("fake API could not decode the request body: %v", err)
	}
	return body
}

func (a *fakeAPI) write(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func records(store map[string]map[string]any) []map[string]any {
	list := make([]map[string]any, 0, len(store))
	for _, record := range store {
		list = append(list, record)
	}
	return list
}

func orDefault(value, fallback any) any {
	if value == nil || value == "" {
		return fallback
	}
	return value
}

// camel maps the snake_case keys some request bodies use onto the camelCase
// keys the corresponding responses carry.
func camel(key string) string {
	switch key {
	case "workspace_id":
		return "workspaceId"
	default:
		return key
	}
}
