package accountsync

import "testing"

func TestValidateMutationSeparatesMessageAndFolderTargets(t *testing.T) {
	base := MutationRequest{ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "folder-action-1"}
	tests := []struct {
		name    string
		request MutationRequest
		valid   bool
	}{
		{name: "message", request: mergeMutation(base, MutationRequest{Kind: MutationSetRead, MessageStableIDs: []string{"INBOX:7:9"}}), valid: true},
		{name: "create", request: mergeMutation(base, MutationRequest{Kind: MutationFolderCreate, ParentStableID: "Projects", FolderName: "日本語", NewFolderPath: "Projects/&ZeVnLIqe-"}), valid: true},
		{name: "rename", request: mergeMutation(base, MutationRequest{Kind: MutationFolderRename, FolderStableID: "Projects/Old", FolderName: "New", FolderPath: "Projects/Old", NewFolderPath: "Projects/New"}), valid: true},
		{name: "delete", request: mergeMutation(base, MutationRequest{Kind: MutationFolderDelete, FolderStableID: "Projects/Old", FolderPath: "Projects/Old"}), valid: true},
		{name: "folder with messages", request: mergeMutation(base, MutationRequest{Kind: MutationFolderDelete, MessageStableIDs: []string{"INBOX:7:9"}, FolderStableID: "Projects", FolderPath: "Projects"})},
		{name: "unresolved create path", request: mergeMutation(base, MutationRequest{Kind: MutationFolderCreate, FolderName: "New"})},
		{name: "rename same path", request: mergeMutation(base, MutationRequest{Kind: MutationFolderRename, FolderStableID: "Old", FolderName: "Old", FolderPath: "Old", NewFolderPath: "Old"})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateMutation(test.request)
			if (err == nil) != test.valid {
				t.Fatalf("ValidateMutation(%#v) error = %v, valid=%v", test.request, err, test.valid)
			}
		})
	}
}

func TestValidateMutationResultsAcceptsOnlyOrderedMoveIdentities(t *testing.T) {
	valid := Mutation{MutationRequest: MutationRequest{Kind: MutationMove, MessageStableIDs: []string{"INBOX:77:9"}}, ResultStableIDs: []string{"Archive:88:41"}}
	if err := ValidateMutationResults(valid); err != nil {
		t.Fatalf("ValidateMutationResults(valid) error = %v", err)
	}
	for _, mutation := range []Mutation{
		{MutationRequest: MutationRequest{Kind: MutationSetRead, MessageStableIDs: []string{"INBOX:77:9"}}, ResultStableIDs: []string{"INBOX:77:9"}},
		{MutationRequest: MutationRequest{Kind: MutationMove, MessageStableIDs: []string{"INBOX:77:9", "INBOX:77:10"}}, ResultStableIDs: []string{"Archive:88:41"}},
		{MutationRequest: MutationRequest{Kind: MutationMove, MessageStableIDs: []string{"INBOX:77:9", "INBOX:77:10"}}, ResultStableIDs: []string{"Archive:88:41", "Archive:88:41"}},
	} {
		if err := ValidateMutationResults(mutation); err == nil {
			t.Fatalf("ValidateMutationResults(%#v) accepted invalid results", mutation)
		}
	}
}

func mergeMutation(base, update MutationRequest) MutationRequest {
	update.ID, update.IdempotencyKey = base.ID, base.IdempotencyKey
	return update
}
