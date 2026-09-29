package app

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "net/http"
 "net/http/httptest"
 "os"
 "strings"
 "testing"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"
)

func workspaceTestService(t *testing.T, applyMigration bool) *Service {
 t.Helper()
 dsn := os.Getenv("WORKSPACE_TEST_DATABASE_URL")
 if dsn == "" { t.Skip("WORKSPACE_TEST_DATABASE_URL is required") }
 ctx := context.Background()
 root,err := pgxpool.New(ctx,dsn)
 if err != nil { t.Fatal(err) }
 schema := fmt.Sprintf("workspace_test_%d",time.Now().UnixNano())
 if _,err := root.Exec(ctx,"create schema "+pgx.Identifier{schema}.Sanitize()); err != nil { root.Close(); t.Fatal(err) }
 t.Cleanup(func() { if _,err := root.Exec(ctx,"drop schema "+pgx.Identifier{schema}.Sanitize()+" cascade"); err != nil { t.Error(err) }; root.Close() })
 cfg,err := pgxpool.ParseConfig(dsn)
 if err != nil { t.Fatal(err) }
 cfg.ConnConfig.RuntimeParams["search_path"] = schema
 cfg.ConnConfig.RuntimeParams["application_name"] = schema
 pool,err := pgxpool.NewWithConfig(ctx,cfg)
 if err != nil { t.Fatal(err) }
 t.Cleanup(pool.Close)
 _,err = pool.Exec(ctx,`create table users(id bigint primary key,email text not null unique,name text not null,enabled boolean not null default true);
 create table api_keys(id uuid primary key default gen_random_uuid(),user_id bigint not null references users(id) on update cascade,revoked_at timestamptz,created_at timestamptz not null default now());
 create table request_logs(id uuid primary key default gen_random_uuid(),user_id bigint references users(id) on update cascade,api_key_id uuid references api_keys(id) on delete set null,created_at timestamptz not null default now());
 create table usage_records(id uuid primary key default gen_random_uuid(),user_id bigint references users(id) on update cascade,api_key_id uuid references api_keys(id) on delete set null,created_at timestamptz not null default now());
 insert into users(id,email,name) select n,'user'||n||'@example.test','User '||n from generate_series(1,6) n;
 insert into api_keys(id,user_id) values('11111111-1111-1111-1111-111111111111',1);
 insert into request_logs(user_id,api_key_id) values(1,'11111111-1111-1111-1111-111111111111'),(2,null);
 insert into usage_records(user_id,api_key_id) values(1,'11111111-1111-1111-1111-111111111111'),(2,null);`)
 if err != nil { t.Fatal(err) }
 if applyMigration {
  sql,err := migrations.ReadFile("migrations/102_workspaces.sql")
  if err != nil { t.Fatal(err) }
  for i:=0;i<2;i++ {
   tx,err := pool.Begin(ctx)
   if err != nil { t.Fatal(err) }
   if _,err := tx.Exec(ctx,string(sql)); err != nil { _ = tx.Rollback(ctx); t.Fatalf("migration run %d: %v",i+1,err) }
   if err := tx.Commit(ctx); err != nil { t.Fatalf("migration commit %d: %v",i+1,err) }
  }
 }
 return &Service{db:pool}
}

func workspaceHandlerRequest(handler http.HandlerFunc,actor,id,member,body string) *httptest.ResponseRecorder {
 r := httptest.NewRequest(http.MethodPost,"/account/workspaces",strings.NewReader(body))
 r.SetPathValue("id",id)
 r.SetPathValue("user_id",member)
 r = r.WithContext(context.WithValue(r.Context(),accountContextKey{},accountContext{userID:actor,role:"admin"}))
 w := httptest.NewRecorder()
 handler(w,r)
 return w
}

func workspaceTestTeam(t *testing.T,s *Service) Workspace {
 t.Helper()
 w := workspaceHandlerRequest(s.createWorkspace,"1","","",`{"name":"Test team","slug":"test-team"}`)
 if w.Code != 201 { t.Fatalf("create=%d %s",w.Code,w.Body.String()) }
 var v Workspace
 if err := json.Unmarshal(w.Body.Bytes(),&v); err != nil { t.Fatal(err) }
 if !validWorkspaceID(v.ID) || v.OwnerID != "1" || v.Role != "owner" || v.IsPersonal || v.CreatedAt.IsZero() || v.UpdatedAt.IsZero() { t.Fatalf("incomplete workspace: %+v",v) }
 for _,m := range []struct { email,role string }{{"user2@example.test","admin"},{"user3@example.test","admin"},{"user4@example.test","member"}} {
  body,_ := json.Marshal(map[string]string{"email":m.email,"role":m.role})
  w := workspaceHandlerRequest(s.addWorkspaceMember,"1",v.ID,"",string(body))
  if w.Code != 201 { t.Fatalf("add=%d %s",w.Code,w.Body.String()) }
  var member workspaceMember
  if err := json.Unmarshal(w.Body.Bytes(),&member); err != nil || member.CreatedAt.IsZero() || member.Email != m.email || member.Name == "" { t.Fatalf("incomplete member: %+v err=%v",member,err) }
 }
 return v
}

func TestWorkspaceMigrationAndDatabaseGuards(t *testing.T) {
 s := workspaceTestService(t,true)
 ctx := context.Background()
 var n int
 if err := s.db.QueryRow(ctx,`select count(*) from workspaces where is_personal`).Scan(&n); err != nil || n != 6 { t.Fatalf("personal count=%d err=%v",n,err) }
 for _,table := range []string{"api_keys","request_logs","usage_records"} {
  if err := s.db.QueryRow(ctx,`select count(*) from `+table+` r join workspaces w on w.id=r.workspace_id where w.is_personal and w.owner_id=r.user_id`).Scan(&n); err != nil || n == 0 { t.Fatalf("%s backfill=%d err=%v",table,n,err) }
  if err := s.db.QueryRow(ctx,`select count(*) from `+table+` where workspace_id is null`).Scan(&n); err != nil || n != 0 { t.Fatalf("%s missing=%d err=%v",table,n,err) }
 }
 if _,err := s.db.Exec(ctx,`insert into users values(7,'user7@example.test','Seven',true)`); err != nil { t.Fatal(err) }
 personal,role,err := s.resolveWorkspace(ctx,"7","")
 if err != nil || role != "owner" || personal == "" { t.Fatalf("automatic personal id=%s role=%s err=%v",personal,role,err) }
 var keyID,keyWorkspace string
 if err := s.db.QueryRow(ctx,`insert into api_keys(user_id) values(7) returning id::text,workspace_id::text`).Scan(&keyID,&keyWorkspace); err != nil || keyWorkspace != personal { t.Fatalf("default key workspace=%s err=%v",keyWorkspace,err) }
 for _,table := range []string{"request_logs","usage_records"} {
  var got string
  if err := s.db.QueryRow(ctx,`insert into `+table+`(user_id,api_key_id) values(7,$1) returning workspace_id::text`,keyID).Scan(&got); err != nil || got != personal { t.Fatalf("%s workspace=%s err=%v",table,got,err) }
 }
 team := workspaceTestTeam(t,s)
 for _,query := range []string{
  `update api_keys set workspace_id=$1 where user_id=7`,
  `insert into api_keys(user_id,workspace_id) values(7,$1)`,
  `update workspace_members set role='member' where workspace_id=$1 and role='owner'`,
  `delete from workspace_members where workspace_id=$1 and role='owner'`,
  `insert into workspace_members(workspace_id,user_id,role) values($1,5,'owner')`,
 } {
  if _,err := s.db.Exec(ctx,query,team.ID); err == nil { t.Fatalf("guard accepted %s",query) }
 }
 if _,err := s.db.Exec(ctx,`update users set id=70 where id=7`); err != nil { t.Fatalf("user ID cascade: %v",err) }
 if got,role,err := s.resolveWorkspace(ctx,"70",""); err != nil || got != personal || role != "owner" { t.Fatalf("cascade personal=%s role=%s err=%v",got,role,err) }
 if _,err := s.db.Exec(ctx,`insert into workspace_members(workspace_id,user_id,role) values($1,1,'member')`,personal); err == nil { t.Fatal("personal workspace accepted another member") }
 if _,err := s.db.Exec(ctx,`update workspaces set archived_at=now() where id=$1`,personal); err == nil { t.Fatal("personal workspace archived") }
}

func TestWorkspaceHandlerPermissionsAndDefaults(t *testing.T) {
 s := workspaceTestService(t,true)
 team := workspaceTestTeam(t,s)
 for _,tc := range []struct { handler http.HandlerFunc; actor,member,body string; want int }{
  {s.addWorkspaceMember,"1","",`{"email":"user1@example.test","role":"member"}`,409},
  {s.addWorkspaceMember,"1","",`{"email":"user4@example.test","role":"admin"}`,409},
  {s.addWorkspaceMember,"2","",`{"email":"user5@example.test","role":"admin"}`,403},
  {s.updateWorkspaceMember,"1","1",`{"role":"member"}`,403},
  {s.updateWorkspaceMember,"2","4",`{"role":"admin"}`,403},
  {s.removeWorkspaceMember,"1","1","",403},
  {s.removeWorkspaceMember,"2","2","",403},
  {s.removeWorkspaceMember,"2","3","",403},
  {s.removeWorkspaceMember,"2","1","",403},
  {s.removeWorkspaceMember,"4","4","",403},
  {s.updateWorkspace,"4","",`{"name":"denied"}`,403},
  {s.updateWorkspace,"2","",`{"slug":"denied"}`,403},
  {s.updateWorkspace,"2","",`{"name":"Allowed"}`,200},
  {s.updateWorkspace,"5","",`{"name":"global-admin"}`,404},
  {s.listWorkspaceMembers,"5","","",404},
  {s.deleteWorkspace,"2","","",403},
  {s.selectWorkspace,"5","","",404},
 } {
  w := workspaceHandlerRequest(tc.handler,tc.actor,team.ID,tc.member,tc.body)
  if w.Code != tc.want { t.Fatalf("actor=%s member=%s body=%s status=%d want=%d response=%s",tc.actor,tc.member,tc.body,w.Code,tc.want,w.Body.String()) }
 }
 if w := workspaceHandlerRequest(s.selectWorkspace,"1",team.ID,"",""); w.Code != 200 { t.Fatal(w.Body.String()) }
 personal,role,err := s.resolveWorkspace(context.Background(),"1","")
 if err != nil || personal == team.ID || role != "owner" { t.Fatalf("select changed default=%s role=%s err=%v",personal,role,err) }
 w := workspaceHandlerRequest(s.listWorkspaces,"1","","","")
 var listed struct { Data []Workspace `json:"data"`; CurrentID string `json:"current_id"` }
 if err := json.Unmarshal(w.Body.Bytes(),&listed); err != nil || listed.CurrentID != personal || len(listed.Data) != 2 { t.Fatalf("list=%s err=%v",w.Body.String(),err) }
 if w := workspaceHandlerRequest(s.deleteWorkspace,"1",personal,"",""); w.Code != 400 { t.Fatalf("delete personal=%d",w.Code) }
 if w := workspaceHandlerRequest(s.addWorkspaceMember,"1",personal,"",`{"email":"user5@example.test","role":"member"}`); w.Code != 400 { t.Fatalf("personal member=%d",w.Code) }
}

func TestWorkspaceArchiveAndMemberRevocation(t *testing.T) {
 s := workspaceTestService(t,true)
 team := workspaceTestTeam(t,s)
 ctx := context.Background()
 var memberKey,ownerKey string
 if err := s.db.QueryRow(ctx,`insert into api_keys(user_id,workspace_id) values(4,$1) returning id::text`,team.ID).Scan(&memberKey); err != nil { t.Fatal(err) }
 if err := s.db.QueryRow(ctx,`insert into api_keys(user_id,workspace_id) values(1,$1) returning id::text`,team.ID).Scan(&ownerKey); err != nil { t.Fatal(err) }
 if _,err := s.db.Exec(ctx,`insert into request_logs(user_id,api_key_id) values(4,$1)`,memberKey); err != nil { t.Fatal(err) }
 if _,err := s.db.Exec(ctx,`insert into usage_records(user_id,api_key_id) values(4,$1)`,memberKey); err != nil { t.Fatal(err) }
 if w := workspaceHandlerRequest(s.removeWorkspaceMember,"2",team.ID,"4",""); w.Code != 204 { t.Fatalf("remove=%d %s",w.Code,w.Body.String()) }
 var revoked bool
 if err := s.db.QueryRow(ctx,`select revoked_at is not null from api_keys where id=$1`,memberKey).Scan(&revoked); err != nil || !revoked { t.Fatalf("member key active err=%v",err) }
 if err := s.db.QueryRow(ctx,`select revoked_at is not null from api_keys where id=$1`,ownerKey).Scan(&revoked); err != nil || revoked { t.Fatalf("owner key revoked prematurely err=%v",err) }
 if _,_,err := s.resolveWorkspace(ctx,"4",team.ID); !errors.Is(err,errWorkspaceAccess) { t.Fatalf("removed member access=%v",err) }
 if _,err := s.db.Exec(ctx,`insert into api_keys(user_id,workspace_id) values(4,$1)`,team.ID); err == nil { t.Fatal("removed member created key") }
 if w := workspaceHandlerRequest(s.deleteWorkspace,"1",team.ID,"",""); w.Code != 204 { t.Fatalf("archive=%d %s",w.Code,w.Body.String()) }
 if err := s.db.QueryRow(ctx,`select revoked_at is not null from api_keys where id=$1`,ownerKey).Scan(&revoked); err != nil || !revoked { t.Fatalf("owner key survives archive err=%v",err) }
 if _,_,err := s.resolveWorkspace(ctx,"1",team.ID); !errors.Is(err,errWorkspaceAccess) { t.Fatalf("archived access=%v",err) }
 if _,err := s.db.Exec(ctx,`insert into api_keys(user_id,workspace_id) values(1,$1)`,team.ID); err == nil { t.Fatal("archived workspace accepted key") }
 for _,table := range []string{"request_logs","usage_records"} {
  var count int
  if err := s.db.QueryRow(ctx,`select count(*) from `+table+` where workspace_id=$1`,team.ID).Scan(&count); err != nil || count != 1 { t.Fatalf("%s history count=%d err=%v",table,count,err) }
  if _,err := s.db.Exec(ctx,`insert into `+table+`(user_id,api_key_id) values(1,$1)`,ownerKey); err != nil { t.Fatalf("in-flight usage after archive rejected: %v",err) }
 }
}

func TestWorkspaceMissingSchemaFailsClosed(t *testing.T) {
 s := workspaceTestService(t,false)
 if id,_,err := s.resolveWorkspace(context.Background(),"1",""); err == nil || id != "" { t.Fatalf("missing schema default id=%s err=%v",id,err) }
 if w := workspaceHandlerRequest(s.listWorkspaces,"1","","",""); w.Code != 500 { t.Fatalf("missing schema status=%d body=%s",w.Code,w.Body.String()) }
}
