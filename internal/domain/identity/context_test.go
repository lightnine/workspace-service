package identity

import "testing"

func TestResolveRelativePath(t *testing.T) {
	t.Parallel()
	ctx := RequestContext{
		OwnerUIN: "100001", UIN: "200001",
		AppID: "260073493", WorkspaceID: "ws-1",
	}.Normalize()

	got, err := ctx.ResolveRelativePath("mini/README.md")
	if err != nil {
		t.Fatal(err)
	}
	want := "260073493/ws-1/users/200001/mini/README.md"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}

	got, err = ctx.ResolveRelativePath(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("full path = %q, want %q", got, want)
	}
}

func TestValidateRequiresWorkspaceFields(t *testing.T) {
	t.Parallel()
	err := RequestContext{OwnerUIN: "1", UIN: "2"}.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
}
