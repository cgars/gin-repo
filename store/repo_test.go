package store

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/G-Node/gin-repo/internal/testbed"
	"io/ioutil"
)

var users UserStore
var repos *RepoStore

var defaultRepo = &RepoId{"foo", "bar"}

var repoids = []struct {
	in  string
	out *RepoId
}{
	//valid repos
	{"foo/bar", defaultRepo},
	{"/foo/bar", defaultRepo},
	{"foo/bar/", defaultRepo},
	{"/foo/bar/", defaultRepo},
	{"/~/foo/bar", defaultRepo},
	{"/~/foo/bar/", defaultRepo},
	{"foo/bar.git", defaultRepo},
	{"/foo/bar.git", defaultRepo},
	{"/~/foo/bar.git", defaultRepo},
	{"/~/foo/bar.git/", defaultRepo},

	//invalid paths
	{"foo", nil},
	{"~/foo/bar/", nil},
	{"//foo//", nil},
	{"/~/foo", nil},
	{"/~//foo/bar", nil},
	{"/foo/bar/se", nil},
	{"foo/bar/se/", nil},
	{"foo/bar/se/foo/bar/se", nil},
	{"//foo//bar//se", nil},
	{"//foo//bar.", nil},
	{"foo//bar.gi", nil},
	{"foo//bar.git.", nil},

	//invalid names
	{"/~foo/bar/", nil},
	{"/foo~/bar/", nil},
	{"/a/b", nil},
}

func TestParseRepoId(t *testing.T) {

	for _, tt := range repoids {
		out, err := RepoIdParse(tt.in)
		if err != nil && tt.out != nil {
			t.Errorf("RepoIdParse(%q) => error, want: %v", tt.in, *tt.out)
		} else if err == nil && tt.out == nil {
			t.Errorf("RepoIdParse(%q) => %v, want error", tt.in, out)
		} else if err == nil && tt.out != nil && !reflect.DeepEqual(out, *(tt.out)) {
			t.Errorf("RepoIdParse(%q) => %v, want %v", tt.in, out, *tt.out)
		}
	}
}

// TestMain sets up a temporary user store for store method tests.
// Currently the temporary files created by this function are not cleaned up.
func TestMain(m *testing.M) {
	repoDir, err := testbed.MkData("/tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not make test data: %v\n", err)
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get cwd: %v", err)
		os.Exit(1)
	}

	// Change to repoDir is required, since creating a local user store depends on
	// auth.ReadSharedSecret(), which reads a file "gin.secret" from the current directory
	// and fails otherwise.
	err = os.Chdir(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not set cwd to repoDir: %v", err)
		os.Exit(1)
	}

	users, err = NewUserStore(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create local user store: %v\n", err)
		os.Exit(1)
	}

	repos, err = NewRepoStore(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create local repo store: %v\n", err)
		os.Exit(1)
	}

	res := m.Run()

	_ = os.Chdir(cwd)
	os.Exit(res)
}

func Test_RepoExists(t *testing.T) {
	const invalidUser = "iDoNotExist"
	const invalidRepo = "iDoNotExist"
	const validUser = "alice"
	const validRepo = "auth"

	// Test empty RepoId
	id := RepoId{"", ""}
	exists, err := repos.RepoExists(id)
	if err != nil {
		t.Fatalf("Unexpected error on empty RepoId: %v\n", err)
	}
	if exists {
		t.Fatal("Did not expect true on empty RepoId.")
	}

	// Test invalid user
	id = RepoId{invalidUser, ""}
	exists, err = repos.RepoExists(id)
	if err != nil {
		t.Fatalf("Unexpected error on invalid user: %v\n", err)
	}
	if exists {
		t.Fatal("Did not expect true on invalid user.")
	}

	// Test missing repo name with existing user
	id = RepoId{validUser, ""}
	exists, err = repos.RepoExists(id)
	if err != nil {
		t.Fatalf("Unexpected error on missing repo: %v\n", err)
	}
	if exists {
		t.Fatal("Did not expect true on missing repo.")
	}

	// Test invalid repo name with existing user
	id = RepoId{validUser, invalidRepo}
	exists, err = repos.RepoExists(id)
	if err != nil {
		t.Fatalf("Unexpected error on invalid repo: %v\n", err)
	}
	if exists {
		t.Fatal("Did not expect true on invalid repo.")
	}

	// Test valid user with valid repository
	id = RepoId{validUser, validRepo}
	exists, err = repos.RepoExists(id)
	if err != nil {
		t.Fatalf("Unexpected error on valid RepoId: %v\n", err)
	}
	if !exists {
		t.Fatal("Did not expect false on valid RepoId.")
	}
}

func TestRepoStore_IdToPath(t *testing.T) {
	const repoOwner = "alice"
	const repoName = "auth"

	r := RepoId{Owner: repoOwner, Name: repoName}
	path := repos.IdToPath(r)

	if !strings.Contains(path, fmt.Sprintf(filepath.Join("repos", "git", repoOwner, repoName+".git"))) {
		t.Fatalf("Received unexpected repository path: %q\n", path)
	}
}

func Test_RepoCreateHooks(t *testing.T) {
	const repoOwner = "alice"
	const repoName = "auth"
	wDir, err := os.Getwd()
	if err != nil{
		t.Logf("Could not determine working directory.%+v", err)
		t.Skip()
		return
	}

	r := RepoId{Owner: repoOwner, Name: repoName}
	// Test for hooks when no dir specified
	err = repos.InitHooks(r)
	if err == nil {
		t.Log("[Ok] Hook without envVar set should not Fail")
	}

	// Test for hooks with dir specified
	err = os.Mkdir(wDir+"/hooktarget", os.ModePerm)
	defer os.Remove(wDir + "/hooktarget")
	if err != nil {
		t.Logf("Could not create hook target %+v", err)
		t.Skip()
		return
	}
	os.Setenv("GIN_HOOK_DIR", filepath.Join(wDir, "hooktarget"))
	err = repos.InitHooks(r)
	if err != nil {
		t.Logf("Hooking went wrong: %+v", err)
		t.Fail()
		return
	}
	fInfo, err := os.Stat(filepath.Join(repos.IdToPath(r), "hooks"))
	if err != nil{
		t.Logf("Cant acess hooks link: %+v", err)
		t.Fail()
		return
	}
	if (fInfo.Mode())&os.ModeSymlink != 0 {
		t.Logf("Hooks is not a link: %+v", fInfo)
		t.Fail()
		return
	}
	t.Logf("[OK] Hooks directory can be linked")
}

func Test_InitRepoMaxSize(t *testing.T) {
	const repoOwner = "alice"
	const repoName = "auth"

	r := RepoId{Owner: repoOwner, Name: repoName}
	rPath := repos.IdToPath(r)
	// Test for hooks when no dir specified
	err := repos.InitRepoMaxSize(r)
	if err != nil{
		t.Logf("Cant init mx Repository size: %+v", err)
		t.Fail()
		return
	}
	f,err := os.Open(filepath.Join(rPath,"gin","size"))
	if err != nil{
		t.Logf("Repos size file was not created: %+v", err)
		t.Fail()
		return
	}

	fCont,err := ioutil.ReadAll(f)
	if err != nil{
		t.Logf("Repo size file could not be read: %+v", err)
		t.Fail()
		return
	}

	if mess:=string(fCont);mess!="maxsize: 5000\ncurrsize: 0\n"{
		t.Logf("Repo size file has wrong content: %s", mess)
		t.Fail()
		return
	}
	t.Logf("[OK] Repo max size can be initialized")
}
