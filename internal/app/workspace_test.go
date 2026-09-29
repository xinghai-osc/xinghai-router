package app

import (
 "strings"
 "testing"
)

func TestWorkspaceValidation(t *testing.T) {
 for _, id := range []string{"00000000-0000-0000-0000-000000000000","aabbccdd-1234-5678-9012-abcdef123456","AABBCCDD-1234-5678-9012-ABCDEF123456"} {
  if !validWorkspaceID(id) { t.Errorf("valid UUID rejected: %q",id) }
 }
 for _, id := range []string{"","aabbccdd123456789012abcdef123456"," aabbccdd-1234-5678-9012-abcdef123456","aabbccdd-1234-5678-9012-abcdef12345g","{aabbccdd-1234-5678-9012-abcdef123456}"} {
  if validWorkspaceID(id) { t.Errorf("invalid UUID accepted: %q",id) }
 }
 for _, name := range []string{"Team",strings.Repeat("星",100)} {
  if !validWorkspaceName(name) { t.Errorf("valid name rejected: %q",name) }
 }
 for _, name := range []string{"",strings.Repeat("星",101),"nul\x00name",string([]byte{0xff})} {
  if validWorkspaceName(name) { t.Errorf("invalid name accepted: %q",name) }
 }
 for _, slug := range []string{"a","team-123",strings.Repeat("a",50)} {
  if !workspaceSlugPattern.MatchString(slug) { t.Errorf("valid slug rejected: %q",slug) }
 }
 for _, slug := range []string{"","-team","team-","Team","星海",strings.Repeat("a",51)} {
  if workspaceSlugPattern.MatchString(slug) { t.Errorf("invalid slug accepted: %q",slug) }
 }
 if got := workspaceSlug("  My Team / 2026  "); got != "my-team-2026" { t.Errorf("slug=%q",got) }
}

func TestWorkspaceRemovalPermission(t *testing.T) {
 for _, tc := range []struct { actor,target string; self,want bool }{
  {"owner","owner",true,false},{"owner","admin",false,true},{"owner","member",false,true},
  {"admin","owner",false,false},{"admin","admin",false,false},{"admin","admin",true,false},
  {"admin","member",false,true},{"admin","member",true,false},{"member","member",true,false},
  {"","member",false,false},{"superadmin","member",false,false},
 } {
  if got := workspaceCanRemove(tc.actor,tc.target,tc.self); got != tc.want { t.Errorf("actor=%s target=%s self=%v got=%v",tc.actor,tc.target,tc.self,got) }
 }
}
