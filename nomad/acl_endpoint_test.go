package nomad

import (
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestACLEndpoint_GetPolicy(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{policy})

	// Lookup the policy
	get := &structs.ACLPolicySpecificRequest{
		Name:         policy.Name,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.SingleACLPolicyResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, policy, resp.Policy)

	// Lookup non-existing policy
	get.Name = structs.GenerateUUID()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Nil(t, resp.Policy)
}

func TestACLEndpoint_GetPolicy_Blocking(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policies
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()

	// First create an unrelated policy
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLPolicies(100, []*structs.ACLPolicy{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the policy we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLPolicies(200, []*structs.ACLPolicy{p2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the policy
	req := &structs.ACLPolicySpecificRequest{
		Name: p2.Name,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleACLPolicyResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Policy == nil || resp.Policy.Name != p2.Name {
		t.Fatalf("bad: %#v", resp.Policy)
	}

	// Eval delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.DeleteACLPolicies(300, []string{p2.Name})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleACLPolicyResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Policy != nil {
		t.Fatalf("bad: %#v", resp2.Policy)
	}
}

func TestACLEndpoint_ListPolicies(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()

	p1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	p2.Name = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{p1, p2})

	// Lookup the policies
	get := &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 2, len(resp.Policies))

	// Lookup the policies by prefix
	get = &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
			Prefix: "aaaabb",
		},
	}
	var resp2 structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp2.Index)
	assert.Equal(t, 1, len(resp2.Policies))
}

func TestACLEndpoint_ListPolicies_Blocking(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policy
	policy := mock.ACLPolicy()

	// Upsert eval triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertACLPolicies(2, []*structs.ACLPolicy{policy}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	assert.Equal(t, uint64(2), resp.Index)
	if len(resp.Policies) != 1 || resp.Policies[0].Name != policy.Name {
		t.Fatalf("bad: %#v", resp.Policies)
	}

	// Eval deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteACLPolicies(3, []string{policy.Name}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 2
	start = time.Now()
	var resp2 structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	assert.Equal(t, uint64(3), resp2.Index)
	assert.Equal(t, 0, len(resp2.Policies))
}

func TestACLEndpoint_DeletePolicies(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{p1})

	// Lookup the policies
	req := &structs.ACLPolicyDeleteRequest{
		Names:        []string{p1.Name},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.DeletePolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
}

func TestACLEndpoint_UpsertPolicies(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()

	// Lookup the policies
	req := &structs.ACLPolicyUpsertRequest{
		Policies:     []*structs.ACLPolicy{p1},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertPolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)

	// Check we created the policy
	out, err := s1.fsm.State().ACLPolicyByName(nil, p1.Name)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestACLEndpoint_UpsertPolicies_Invalid(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	p1.Rules = "blah blah invalid"

	// Lookup the policies
	req := &structs.ACLPolicyUpsertRequest{
		Policies:     []*structs.ACLPolicy{p1},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertPolicies", req, &resp)
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("bad: %s", err)
	}
}

func TestACLEndpoint_GetToken(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	token := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{token})

	// Lookup the token
	get := &structs.ACLTokenSpecificRequest{
		AccessorID:   token.AccessorID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.SingleACLTokenResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, token, resp.Token)

	// Lookup non-existing token
	get.AccessorID = structs.GenerateUUID()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Nil(t, resp.Token)
}

func TestACLEndpoint_GetToken_Blocking(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the tokens
	p1 := mock.ACLToken()
	p2 := mock.ACLToken()

	// First create an unrelated token
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLTokens(100, []*structs.ACLToken{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the token we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLTokens(200, []*structs.ACLToken{p2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the token
	req := &structs.ACLTokenSpecificRequest{
		AccessorID: p2.AccessorID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleACLTokenResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Token == nil || resp.Token.AccessorID != p2.AccessorID {
		t.Fatalf("bad: %#v", resp.Token)
	}

	// Eval delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.DeleteACLTokens(300, []string{p2.AccessorID})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleACLTokenResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Token != nil {
		t.Fatalf("bad: %#v", resp2.Token)
	}
}

func TestACLEndpoint_ListTokens(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	p2 := mock.ACLToken()

	p1.AccessorID = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	p2.AccessorID = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{p1, p2})

	// Lookup the tokens
	get := &structs.ACLTokenListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 2, len(resp.Tokens))

	// Lookup the tokens by prefix
	get = &structs.ACLTokenListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
			Prefix: "aaaabb",
		},
	}
	var resp2 structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp2.Index)
	assert.Equal(t, 1, len(resp2.Tokens))
}

func TestACLEndpoint_ListTokens_Blocking(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the token
	token := mock.ACLToken()

	// Upsert eval triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertACLTokens(2, []*structs.ACLToken{token}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.ACLTokenListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	assert.Equal(t, uint64(2), resp.Index)
	if len(resp.Tokens) != 1 || resp.Tokens[0].AccessorID != token.AccessorID {
		t.Fatalf("bad: %#v", resp.Tokens)
	}

	// Eval deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteACLTokens(3, []string{token.AccessorID}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 2
	start = time.Now()
	var resp2 structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	assert.Equal(t, uint64(3), resp2.Index)
	assert.Equal(t, 0, len(resp2.Tokens))
}

func TestACLEndpoint_DeleteTokens(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{p1})

	// Lookup the tokens
	req := &structs.ACLTokenDeleteRequest{
		AccessorIDs:  []string{p1.AccessorID},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.DeleteTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
}

func TestACLEndpoint_UpsertTokens(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()

	// Lookup the tokens
	req := &structs.ACLTokenUpsertRequest{
		Tokens:       []*structs.ACLToken{p1},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)

	// Check we created the token
	out, err := s1.fsm.State().ACLTokenByPublicID(nil, p1.AccessorID)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestACLEndpoint_UpsertTokens_Invalid(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	p1.Type = "blah blah"

	// Lookup the tokens
	req := &structs.ACLTokenUpsertRequest{
		Tokens:       []*structs.ACLToken{p1},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertTokens", req, &resp)
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "client or management") {
		t.Fatalf("bad: %s", err)
	}
}
