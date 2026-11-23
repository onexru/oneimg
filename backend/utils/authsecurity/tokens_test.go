package authsecurity

import (
	"reflect"
	"testing"

	"oneimg/backend/models"
)

func apiTokenWithScopes(scopes string) models.APICredential {
	return models.APICredential{Scopes: scopes}
}

// PermissionCodes 决定 token 能被控制器认可哪些权限码。写死列表时
// 「路由放行、控制器 403」的错配就出在这里。
func TestPermissionCodes(t *testing.T) {
	cases := []struct {
		scopes string
		want   []string
	}{
		// 默认 scope：读、写、删三件套。
		{"image:read,upload:write,image:delete", []string{"image:read", "upload:write", "image:delete"}},
		// 老 token 只存了前两个，删除应当仍然不可用（不静默提权）。
		{"image:read,upload:write", []string{"image:read", "upload:write"}},
		// 空格、重复、未知值都要容错。
		{" image:delete , image:delete ,bogus", []string{"image:delete"}},
		{"*,image:read", []string{"image:read"}},
		{"", []string{}},
		{"   ", []string{}},
	}

	for _, c := range cases {
		got := PermissionCodes(c.scopes)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("PermissionCodes(%q) = %v, want %v", c.scopes, got, c.want)
		}
	}
}

func TestDefaultTokenScopesMatchesCatalog(t *testing.T) {
	if got := PermissionCodes(DefaultTokenScopes); len(got) != len(TokenScopes) {
		t.Fatalf("默认 scopes %q 未覆盖全部 %v，实际得到 %v", DefaultTokenScopes, TokenScopes, got)
	}
	for _, scope := range TokenScopes {
		if !HasScope(apiTokenWithScopes(DefaultTokenScopes), scope) {
			t.Errorf("默认 token 缺少 scope %q", scope)
		}
	}
}
