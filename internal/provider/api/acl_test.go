package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

func TestClientListACLsCachesPerVirtualCluster(t *testing.T) {
	t.Parallel()

	state := newACLTestServerState(map[string][]ACLResponse{
		"vc-1": {testACLResponse("orders", "User:alice", "READ")},
		"vc-2": {testACLResponse("payments", "User:bob", "WRITE")},
	})

	server := newACLTestServer(state)
	defer server.Close()

	client := newACLTestClient(t, server.URL)

	first, err := client.ListACLs("vc-1")
	if err != nil {
		t.Fatalf("ListACLs(vc-1) returned error: %v", err)
	}

	second, err := client.ListACLs("vc-1")
	if err != nil {
		t.Fatalf("ListACLs(vc-1) cached call returned error: %v", err)
	}

	other, err := client.ListACLs("vc-2")
	if err != nil {
		t.Fatalf("ListACLs(vc-2) returned error: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected cached ACLs to match first response, got %v and %v", first, second)
	}

	if len(other) != 1 || other[0].ResourceName != "payments" {
		t.Fatalf("expected vc-2 ACLs to come from a separate cache entry, got %v", other)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if got := state.listCalls["vc-1"]; got != 1 {
		t.Fatalf("expected vc-1 ACLs to be fetched once, got %d list calls", got)
	}

	if got := state.listCalls["vc-2"]; got != 1 {
		t.Fatalf("expected vc-2 ACLs to be fetched once, got %d list calls", got)
	}
}

func TestClientGetACLUsesCacheIndex(t *testing.T) {
	t.Parallel()

	targetACL := testACLResponse("orders", "User:alice", "READ")
	state := newACLTestServerState(map[string][]ACLResponse{
		"vc-1": {
			targetACL,
			testACLResponse("payments", "User:bob", "WRITE"),
		},
	})

	server := newACLTestServer(state)
	defer server.Close()

	client := newACLTestClient(t, server.URL)

	if _, err := client.ListACLs("vc-1"); err != nil {
		t.Fatalf("ListACLs returned error: %v", err)
	}

	client.aclsCache.mu.Lock()
	entry, ok := client.aclsCache.entriesByVC["vc-1"]
	client.aclsCache.mu.Unlock()

	if !ok {
		t.Fatal("expected ACL cache entry to be populated for vc-1")
	}

	if len(entry.aclsByID) != 2 {
		t.Fatalf("expected ACL cache index to contain 2 entries, got %d", len(entry.aclsByID))
	}

	indexedACL, found := entry.aclsByID[ACLRequest(targetACL).ID()]
	if !found {
		t.Fatal("expected ACL cache index to contain the target ACL")
	}

	if indexedACL != targetACL {
		t.Fatalf("expected indexed ACL to match target ACL, got %v", indexedACL)
	}

	gotACL, err := client.GetACL("vc-1", ACLRequest(targetACL))
	if err != nil {
		t.Fatalf("GetACL returned error: %v", err)
	}

	if *gotACL != targetACL {
		t.Fatalf("expected GetACL to return %v, got %v", targetACL, *gotACL)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if got := state.listCalls["vc-1"]; got != 1 {
		t.Fatalf("expected GetACL to use the warmed cache, got %d list calls", got)
	}
}

func TestClientCreateAndDeleteACLInvalidateCache(t *testing.T) {
	t.Parallel()

	existingACL := testACLResponse("orders", "User:alice", "READ")
	createdACL := testACLRequest("orders", "User:alice", "WRITE")

	state := newACLTestServerState(map[string][]ACLResponse{
		"vc-1": {existingACL},
	})

	server := newACLTestServer(state)
	defer server.Close()

	client := newACLTestClient(t, server.URL)

	if _, err := client.ListACLs("vc-1"); err != nil {
		t.Fatalf("initial ListACLs returned error: %v", err)
	}

	if _, err := client.ListACLs("vc-1"); err != nil {
		t.Fatalf("cached ListACLs returned error: %v", err)
	}

	if _, err := client.CreateACL("vc-1", createdACL); err != nil {
		t.Fatalf("CreateACL returned error: %v", err)
	}

	aclsAfterCreate, err := client.ListACLs("vc-1")
	if err != nil {
		t.Fatalf("ListACLs after CreateACL returned error: %v", err)
	}

	if len(aclsAfterCreate) != 2 {
		t.Fatalf("expected 2 ACLs after create, got %d", len(aclsAfterCreate))
	}

	if !containsACL(aclsAfterCreate, ACLResponse(createdACL)) {
		t.Fatalf("expected created ACL to be returned after cache invalidation, got %v", aclsAfterCreate)
	}

	if err := client.DeleteACL("vc-1", ACLRequest(existingACL)); err != nil {
		t.Fatalf("DeleteACL returned error: %v", err)
	}

	aclsAfterDelete, err := client.ListACLs("vc-1")
	if err != nil {
		t.Fatalf("ListACLs after DeleteACL returned error: %v", err)
	}

	if len(aclsAfterDelete) != 1 {
		t.Fatalf("expected 1 ACL after delete, got %d", len(aclsAfterDelete))
	}

	if containsACL(aclsAfterDelete, existingACL) {
		t.Fatalf("expected deleted ACL to be absent after cache invalidation, got %v", aclsAfterDelete)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if got := state.listCalls["vc-1"]; got != 3 {
		t.Fatalf("expected ListACLs to refetch after create and delete, got %d list calls", got)
	}

	if got := state.createCalls["vc-1"]; got != 1 {
		t.Fatalf("expected 1 create call, got %d", got)
	}

	if got := state.deleteCalls["vc-1"]; got != 1 {
		t.Fatalf("expected 1 delete call, got %d", got)
	}
}

type aclTestServerState struct {
	mu          sync.Mutex
	aclsByVC    map[string][]ACLResponse
	listCalls   map[string]int
	createCalls map[string]int
	deleteCalls map[string]int
}

func newACLTestServerState(initial map[string][]ACLResponse) *aclTestServerState {
	state := &aclTestServerState{
		aclsByVC:    make(map[string][]ACLResponse, len(initial)),
		listCalls:   make(map[string]int, len(initial)),
		createCalls: make(map[string]int, len(initial)),
		deleteCalls: make(map[string]int, len(initial)),
	}

	for vcID, acls := range initial {
		state.aclsByVC[vcID] = cloneACLResponses(acls)
	}

	return state
}

func newACLTestServer(state *aclTestServerState) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/virtual_clusters/acls/list":
			var req ACLListRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			state.mu.Lock()
			state.listCalls[req.VirtualClusterID]++
			acls := cloneACLResponses(state.aclsByVC[req.VirtualClusterID])
			state.mu.Unlock()

			writeACLTestResponse(w, ACLListResponse{ACLs: acls})
		case "/virtual_clusters/acls/create":
			var req ACLCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			createdACL := ACLResponse(req.ACL)

			state.mu.Lock()
			state.createCalls[req.VirtualClusterID]++
			state.aclsByVC[req.VirtualClusterID] = append(state.aclsByVC[req.VirtualClusterID], createdACL)
			state.mu.Unlock()

			writeACLTestResponse(w, ACLDescribeResponse{ACL: createdACL})
		case "/virtual_clusters/acls/delete":
			var req ACLDeleteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			state.mu.Lock()
			state.deleteCalls[req.VirtualClusterID]++

			currentACLs := state.aclsByVC[req.VirtualClusterID]
			remainingACLs := make([]ACLResponse, 0, len(currentACLs))
			deletedACLs := make([]ACLResponse, 0, len(req.ACLs))

			for _, currentACL := range currentACLs {
				shouldDelete := false
				for _, aclToDelete := range req.ACLs {
					if aclsEqual(aclToDelete, currentACL) {
						shouldDelete = true
						deletedACLs = append(deletedACLs, currentACL)
						break
					}
				}

				if !shouldDelete {
					remainingACLs = append(remainingACLs, currentACL)
				}
			}

			state.aclsByVC[req.VirtualClusterID] = remainingACLs
			state.mu.Unlock()

			writeACLTestResponse(w, ACLDeleteResponse{ACLs: deletedACLs})
		default:
			http.NotFound(w, r)
		}
	}))
}

func newACLTestClient(t *testing.T, host string) *Client {
	t.Helper()

	token := "test-token"
	client, err := NewClient(host, &token)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	return client
}

func testACLRequest(resourceName, principal, operation string) ACLRequest {
	return ACLRequest{
		ResourceType:   "TOPIC",
		ResourceName:   resourceName,
		PatternType:    "LITERAL",
		Principal:      principal,
		Host:           "*",
		Operation:      operation,
		PermissionType: "ALLOW",
	}
}

func testACLResponse(resourceName, principal, operation string) ACLResponse {
	return ACLResponse(testACLRequest(resourceName, principal, operation))
}

func containsACL(acls []ACLResponse, target ACLResponse) bool {
	for _, acl := range acls {
		if acl == target {
			return true
		}
	}

	return false
}

func writeACLTestResponse(w http.ResponseWriter, response any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
