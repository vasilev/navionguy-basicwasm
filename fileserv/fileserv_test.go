package fileserv

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/navionguy/basicwasm/gwtoken"
	"github.com/navionguy/basicwasm/object"
	"github.com/stretchr/testify/assert"
)

const (
	sawOpen    = "sawOpen"
	sawReadDir = "sawReadDir"
	sawStat    = "sawStat"
	sawName    = "sawName"
)

type mockFS struct {
	file       string // filename
	statErr    bool   // return an error when stat is called
	readErr    *bool  // return error from read call
	openAlways bool   // return a file handle no matter what
	events     map[string]bool

	// desired Readdir results
	names []string
	err   int
}

func (mf mockFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

func (mf mockFS) Open(file string) (http.File, error) {
	if mf.events != nil {
		mf.events[sawOpen] = true
	}
	if (mf.file != file) && !mf.openAlways {
		return nil, fmt.Errorf("got %s, wanted %s", file, mf.file)
	}
	return mf, nil
}

func (mf mockFS) Read(p []byte) (int, error) {

	if *mf.readErr {
		return 0, io.EOF
	}
	if len(mf.file) > 0 {
		l := len(p)
		if len(mf.file) < l {
			l = len(mf.file)
			*mf.readErr = true // he has read it all
		}
		rc := copy(p, []byte(mf.file[:l]))
		return rc, nil
	}

	return 0, nil
}

func (mf mockFS) Readdir(n int) ([]os.FileInfo, error) {
	if mf.events != nil {
		mf.events[sawReadDir] = true
	}
	if mf.err != http.StatusOK {
		return nil, io.EOF
	}

	var mi []os.FileInfo
	for _, nm := range mf.names {
		nmi := mockFI{name: nm, mom: &mf}
		mi = append(mi, nmi)
	}

	return mi, nil
}

func (mf mockFS) Seek(offset int64, whence int) (int64, error) {
	var rc int64
	switch whence {
	case io.SeekEnd:
		rc = int64(len(mf.file))
		if len(mf.names) > 0 {
			rc = int64(len(mf.names))
		}
	case io.SeekStart:
		rc = 0
	}
	return rc, nil
}

func (mf mockFS) Stat() (os.FileInfo, error) {
	if mf.events != nil {
		mf.events[sawStat] = true
	}
	if mf.statErr {
		return nil, errors.New("a faked error")
	}

	nmi := mockFI{name: mf.file, mom: &mf}

	for _, f := range mf.names {
		nmi.files = append(nmi.files, f)
	}

	return nmi, nil
}

func (mf *mockFS) SawName() {
	if mf.events != nil {
		mf.events[sawName] = true
	}
}

func (mf mockFS) Close() error {
	return nil
}

type mockFI struct {
	name  string
	files []string
	mom   *mockFS
}

func (mi mockFI) IsDir() bool {
	if len(mi.files) > 1 {
		return true
	}
	return false
}

func (mi mockFI) ModTime() time.Time {
	return time.Now()
}

func (mi mockFI) Mode() os.FileMode {
	return os.ModeDir
}

func (mi mockFI) Name() string {
	if mi.mom != nil {
		mi.mom.SawName()
	}
	return mi.name
}

func (mi mockFI) Size() int64 {
	return int64(len(mi.name))
}

func (mi mockFI) Sys() interface{} {
	return nil
}

type mockTerm struct {
	row     *int
	col     *int
	strVal  *string
	sawStr  *string
	sawCls  *bool
	sawBeep *bool
}

func initMockTerm(mt *mockTerm) {
	mt.row = new(int)
	*mt.row = 0

	mt.col = new(int)
	*mt.col = 0

	mt.strVal = new(string)
	*mt.strVal = ""

	mt.sawCls = new(bool)
	*mt.sawCls = false
}

func (mt mockTerm) Cls() {
	*mt.sawCls = true
}

func (mt mockTerm) Print(msg string) {
	fmt.Print(msg)
}

func (mt mockTerm) Println(msg string) {
	fmt.Println(msg)
	if mt.sawStr != nil {
		*mt.sawStr = *mt.sawStr + msg
	}
}

func (mt mockTerm) SoundBell() {
	fmt.Print("\x07")
	*mt.sawBeep = true
}

func (mt mockTerm) Locate(int, int) {
}

func (mt mockTerm) GetCursor() (int, int) {
	return *mt.row, *mt.col
}

func (mt mockTerm) Read(col, row, len int) string {
	// make sure your test is correct
	trim := (row-1)*80 + (col - 1)

	tstr := *mt.strVal

	newstr := tstr[trim : trim+len]

	return newstr
}

func (mt mockTerm) ReadKeys(count int) []byte {
	if mt.strVal == nil {
		return nil
	}

	bt := []byte(*mt.strVal)

	if count >= len(bt) {
		mt.strVal = nil
		return bt
	}

	v := (*mt.strVal)[:count]
	mt.strVal = &v

	return bt[:count]
}

func Test_WrapSource(t *testing.T) {
	rt := mux.NewRouter()
	fs := fileSource{src: http.Dir("../source")}
	fs.wrapSource(rt, "/driveC/{file}.{ext}", "text/plain; charset=ASCII")

	ts := httptest.NewServer(rt)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/driveC/start.bas")

	assert.Nil(t, err, "http.Get got error")
	assert.NotEmpty(t, res, "http.Get no body returned")
}

func Test_WrapSubDirs(t *testing.T) {
	tests := []struct {
		tname      string
		fname      string
		rc         int
		files      []string
		events     []string
		openAlways bool
		statErr    bool
	}{
		{tname: "WrapSubDirs fail stat", fname: "/", openAlways: true, rc: http.StatusOK, files: []string{"test.bas"}, events: []string{"sawOpen", "sawReadDir", "sawName", "sawStat"}, statErr: true},
		{tname: "WrapSubDirs fail fopen", fname: "/", rc: http.StatusOK, files: []string{"test.bas"}, events: []string{"sawOpen", "sawReadDir", "sawName"}},
		{tname: "WrapSubDirs fopen", fname: "/", openAlways: true, rc: http.StatusOK, files: []string{"test.bas"}, events: []string{"sawOpen", "sawReadDir", "sawName", "sawStat"}},
		{tname: "WrapSubDirs fail dotfile", fname: "/", rc: http.StatusOK, files: []string{".test"}, events: []string{"sawOpen", "sawReadDir", "sawName"}},
		{tname: "WrapSubDirs fail readdir", fname: "/", rc: http.StatusTeapot, events: []string{"sawOpen", "sawReadDir"}},
		{tname: "WrapSubDirs fail open", fname: "bogus", rc: http.StatusOK, events: []string{"sawOpen"}},
	}

	for _, tt := range tests {
		fs := mockFS{file: tt.fname, err: tt.rc, names: tt.files, openAlways: tt.openAlways, statErr: tt.statErr}
		fs.events = make(map[string]bool)
		src := fileSource{src: fs}
		rt := mux.NewRouter()
		src.wrapSubDirs(rt, "/", "/")

		assert.Equal(t, len(tt.events), len(fs.events), "Test %s unexpectedly got %d events", tt.tname, len(fs.events))
	}
}

func Test_WrapFileSources(t *testing.T) {
	rt := mux.NewRouter()
	fix := "../source"
	drives["driveC"] = &fix

	WrapFileSources(rt)

	for key, drv := range drives {

		// if drive has a path, make sure it has a route
		if len(*drv) > 0 {
			trt := rt.Get("/" + key)
			assert.NotEmpty(t, trt, "drive %s failed to get a route\n", key)

			path, _ := trt.GetPathRegexp()
			assert.Contains(t, path, key, "route doesn't include key")

			ts := httptest.NewServer(rt)
			defer ts.Close()

			res, err := http.Get(ts.URL + "/driveC/")

			assert.Nil(t, err, "http.Get got error")
			assert.NotEmpty(t, res, "http.Get no body returned")
		}
	}
}

func Test_ContainsDotFile(t *testing.T) {
	tests := []struct {
		name   string
		expect bool
	}{
		{name: "menu.bas", expect: false},
		{name: ".gitignore", expect: true},
		{name: "html/../main.html", expect: true},
	}

	for _, tt := range tests {
		if containsDotFile(tt.name) != tt.expect {
			t.Fatalf("%s should have gotten %T but got %T\n", tt.name, tt.expect, containsDotFile(tt.name))
		}
	}
}

func Test_Open(t *testing.T) {
	tests := []struct {
		name string
		want string
		fail bool
	}{
		{name: ".gitignore", fail: true},
		{name: "menu", want: "menu", fail: false},
		{name: "menu", want: "hello", fail: true},
	}

	for _, tt := range tests {
		ifs := mockFS{file: tt.want}
		ifs.events = make(map[string]bool)

		fs := fileSource{src: ifs}

		_, err := fs.Open(tt.name)
		if (err != nil) != tt.fail {
			t.Fatalf("Open(%s) should have gotten %T but got %T\n", tt.name, tt.fail, (err != nil))
		}
	}
}

func Test_SendDirectory(t *testing.T) {
	tests := []struct {
		files []string
		want  string
		mtype string
		res   int
	}{
		{files: []string{"hello.bas", "menu.bas"}, want: "", res: 404},
		{files: []string{"hello.bas", ".gitignore", "menu.bas"}, want: `[{"name":"hello.bas","isdir":false},{"name":"menu.bas","isdir":false}]`, res: 200},
		{files: []string{"hello.bas", "menu.bas"}, want: `[{"name":"hello.bas","isdir":false},{"name":"menu.bas","isdir":false}]`, res: 200},
	}

	for _, tt := range tests {
		fs := mockFS{err: tt.res}
		fs.events = make(map[string]bool)
		for _, tf := range tt.files {
			fs.names = append(fs.names, tf)
		}
		ffs := fileSource{}
		df := dotFileHidingFile{fs}
		rr := httptest.NewRecorder()

		ffs.sendDirectory(df, rr)

		bufstr := validateResult(t, rr, tt.res, tt.mtype)
		assert.EqualValues(t, bufstr, tt.want, "got result %s\n wanted %s\n", bufstr, tt.want)
	}
}

func validateResult(t *testing.T, rr *httptest.ResponseRecorder, rc int, mtype string) string {
	if rr.Result().StatusCode != rc {
		t.Fatalf("got status %d wanted %d\n", rr.Result().StatusCode, rc)
	}

	if rr.Body.Len() == 0 {
		return ""
	}

	buf := make([]byte, rr.Body.Len())
	_, err := io.ReadFull(rr.Body, buf)

	assert.Nil(t, err, "validate result got an error trying to read body\n")

	if len(mtype) > 0 {
		assert.Equal(t, mtype, rr.HeaderMap.Get("content-type"), "expected mime type %s, got %s", mtype, rr.HeaderMap.Get("content-type"))
	}

	return string(buf)
}

func Test_ServeFile(t *testing.T) {
	tests := []struct {
		testid  string
		fname   string
		mtype   string
		res     int
		want    string
		statErr bool
		readErr bool
		files   []string
	}{
		{testid: "read fail", fname: "hello.bas", mtype: "text/plain; charset=ASCII", res: 503, want: "", readErr: true},
		{testid: "dir", fname: "/", mtype: "application/json", res: 200,
			want:  `[{"name":"hello.bas","isdir":false},{"name":"test.bas","isdir":false},{"name":"menu.bas","isdir":false}]`,
			files: []string{"hello.bas", "test.bas", "menu.bas"}},
		{testid: "stat Error", fname: "hello.bas", res: 500, want: "", statErr: true},
		{testid: "file not found", fname: "hello.bas", res: 404, want: ""},
		{testid: "read from root", fname: "", res: 200, mtype: "text/plain; charset=ASCII", want: "/"},
		{testid: "read file", fname: "hello.bas", mtype: "text/plain; charset=ASCII", res: 200, want: "hello.bas"},
	}

	for _, tt := range tests {
		fs := mockFS{file: tt.fname, err: tt.res, statErr: tt.statErr, readErr: &tt.readErr}
		fs.events = make(map[string]bool)
		for _, name := range tt.files {
			fs.names = append(fs.names, name)
		}
		// setup certain errors
		if len(tt.fname) == 0 {
			fs.file = tt.want // empty name should be treated as root
		}
		if tt.res == 404 {
			fs.file = "" // no known files throws an error
		}

		rr := httptest.NewRecorder()
		src := fileSource{src: fs}
		req, err := http.NewRequest("GET", tt.fname, nil)
		assert.Nilf(t, err, "Build rqst failed")
		src.serveFile(rr, req, tt.fname, tt.mtype)

		if rr.Result().StatusCode != tt.res {
			t.Fatalf("got status %d wanted %d\n", rr.Result().StatusCode, tt.res)
		}

		bufstr := validateResult(t, rr, tt.res, tt.mtype)

		if strings.Compare(bufstr, tt.want) != 0 {
			t.Fatalf("got result: %s\nwanted : %s\n", bufstr, tt.want)
		}
	}
}

func Test_Readdir(t *testing.T) {

	tests := []struct {
		name   string
		err    string
		fnames []string
		want   []string
	}{
		{name: "test file list", fnames: []string{"hello.bas", ".gitignore"}, want: []string{"hello.bas"}},
		{name: "test error handling", err: "test error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := new(mockFS)
			fs.events = make(map[string]bool)

			if len(tt.err) > 0 {
				//	fs.err = errors.New(tt.err)
			}

			for _, nm := range tt.fnames {
				fs.names = append(fs.names, nm)
			}

			dfs := dotFileHidingFile{*fs}
			dfs.Readdir(-1)
		})
	}
}

// testing client side routines

func Test_SetDirTag(t *testing.T) {
	tests := []struct {
		flag bool
		exp  string
	}{
		{false, "    "},
		{true, "<dir>"},
	}

	for _, tt := range tests {
		res := setDirTag(tt.flag)

		assert.Zero(t, strings.Compare(tt.exp, res), "Expected %s got %s", tt.exp, res)
	}
}

func Test_FormatExtension(t *testing.T) {
	tests := []struct {
		ext string
		exp string
	}{
		{"bas", "bas"},
		{"basic", "bas"},
		{"bas.tmp", "bas"},
	}

	for _, tt := range tests {
		res := formatExtension(tt.ext)

		assert.Zero(t, strings.Compare(tt.exp, res), "Expected %s got %s", tt.exp, res)
	}
}

func Test_FormatBaseName(t *testing.T) {
	tests := []struct {
		name string
		exp  string
	}{
		{"basic", "basic"},
		{"longername", "longern+"},
		{"exactly8", "exactly8"},
	}

	for _, tt := range tests {
		res := formatBaseName(tt.name)

		assert.Zero(t, strings.Compare(tt.exp, res), "Expected %s got %s", tt.exp, res)
	}
}

func Test_FormatFileName(t *testing.T) {
	tests := []struct {
		name string
		flag bool
		exp  string
	}{
		{"basic", false, "basic   .       "},
		{"basic.bas", false, "basic   .bas    "},
		{"longername.basic", false, "longern+.bas    "},
		{"exactly8.bas", false, "exactly8.bas    "},
		{"exactly8.bas", true, "exactly8.bas<dir>"},
	}

	for _, tt := range tests {
		res := FormatFileName(tt.name, tt.flag)

		assert.Zero(t, strings.Compare(tt.exp, res), "Test_FormatFileName() expected %s got %s", tt.exp, res)
	}
}

func Test_CheckForDrive(t *testing.T) {
	tests := []struct {
		path string
		exp  bool
	}{
		{"/", false},
		{"c:/", true},
		{"menu", false},
	}

	for _, tt := range tests {
		res := checkForDrive(tt.path)

		assert.Equal(t, tt.exp, res, "Test_CheckForDrive expected %t got %t", tt.exp, res)
	}
}

func Test_ConvertDrive(t *testing.T) {
	tests := []struct {
		path string
		cwd  string
		exp  string
	}{
		{``, `c:`, "driveC"},
		{`C:\`, `c:`, "driveC"},
		{`c:\`, `c:\`, "driveC"},
		{`\menu`, `c:\`, "driveC/menu"},
		{`prog\test.bas`, "c:/menu", "driveC/menu/prog/test.bas"},
	}

	for _, tt := range tests {
		res := convertDrive(tt.path, tt.cwd)

		assert.Equal(t, tt.exp, res, "Test_ConvertDrive expected %s, got %s", tt.exp, res)
	}
}

func Test_GetCWD(t *testing.T) {
	tests := []struct {
		cwd string
		exp string
	}{
		{"", `C:\`},
		{`D:\menu`, `D:\menu`},
	}

	for _, tt := range tests {
		var trm object.Console
		env := object.NewTermEnvironment(trm)

		if len(tt.cwd) > 0 {
			drv := object.String{Value: tt.cwd}
			env.Set(object.WORK_DRIVE, &drv)
		}

		res := GetCWD(env)

		assert.Equal(t, tt.exp, res, "Test_GetCWD fail, expected %s got %s", tt.exp, res)
	}
}

func Test_GetURL(t *testing.T) {
	tests := []struct {
		url string
		exp string
	}{
		{"", "http://localhost:8080/"},
		{"https://gwbasic:3002", "https://gwbasic:3002/"},
	}

	for _, tt := range tests {
		var trm object.Console
		env := object.NewTermEnvironment(trm)

		if len(tt.url) > 0 {
			url := object.String{Value: tt.url}
			env.Set(object.SERVER_URL, &url)
		}

		res := getURL(env)

		assert.Equal(t, tt.exp, res, "Test_GetURL fail, expected %s got %s", tt.exp, res)
	}
}

func Test_BuildRequestURL(t *testing.T) {
	tests := []struct {
		url  string
		cwd  string
		file string
		exp  string
	}{
		{"http://localhost:8080/", `C:\`, "menu1.bas", "http://localhost:8080/driveC/menu1.bas"},
		{"http://localhost:8080/", `C:\`, `prog\menu1.bas`, "http://localhost:8080/driveC/prog/menu1.bas"},
	}

	for _, tt := range tests {
		var trm object.Console
		env := object.NewTermEnvironment(trm)

		if len(tt.url) > 0 {
			url := object.String{Value: tt.url}
			env.Set(object.SERVER_URL, &url)
		}

		if len(tt.cwd) > 0 {
			drv := object.String{Value: tt.cwd}
			env.Set(object.WORK_DRIVE, &drv)
		}

		res := buildRequestURL(tt.file, env)

		assert.Equal(t, tt.exp, res, "Test_BuildRequestURL fail, expected %s got %s", tt.exp, res)
	}
}

func Test_GetFile(t *testing.T) {
	tests := []struct {
		url  string
		cwd  string
		file string
		send string
		exp  string
		rs   int
		err  bool
	}{
		{``, `C:\`, `menu\menu1.bas`, "10 PRINT \"Main Menu\"\n", "10 PRINT \"Main Menu\"\n", 200, false},
		{`http://localhost:4321`, `C:\`, `menu\menu1.bas`, "10 PRINT \"Main Menu\"\n", "", 200, true},
		{``, `C:\`, `menu\menu1.bas`, "", "", 404, true},
	}

	for _, tt := range tests {
		var trm object.Console
		env := object.NewTermEnvironment(trm)
		ts := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(tt.rs)
			res.Write([]byte(tt.send))
		}))
		defer ts.Close()

		url := object.String{Value: ts.URL}
		if len(tt.url) > 0 {
			url = object.String{Value: tt.url}
		}
		env.Set(object.SERVER_URL, &url)

		if len(tt.cwd) > 0 {
			drv := object.String{Value: tt.cwd}
			env.Set(object.WORK_DRIVE, &drv)
		}

		bt, err := GetFile(tt.file, env)

		if !tt.err {
			assert.NoError(t, err, "Test_GetFile failed with error")
		} else {
			assert.Error(t, err, "Test_GetFile succeeded will expecting error")
		}

		if len(tt.exp) > 0 {
			resb, err := ioutil.ReadAll(bt)

			if err == nil {
				res := string(resb)

				assert.Equal(t, tt.exp, res, "Test_GetFile fail, expected %s got %s", tt.exp, res)
			}
		}
	}
}

func Test_ParseFile(t *testing.T) {
	tests := []struct {
		inp   []byte
		stmts int
	}{
		{inp: []byte{}},
		{inp: []byte{gwtoken.TOKEN_FILE, 0x7C, 0x12, 0x0A, 0x00, 0x91, 0x20, 0x22, 0x48, 0x65, 0x6C,
			0x6C, 0x6F, 0x22, 0x00, 0x87, 0x12, 0x14, 0x00, 0x59, 0x20, 0xE7,
			0x20, 0x0F, 0x96, 0x00, 0x92, 0x12, 0x1E, 0x00, 0x5A, 0x20, 0xE7,
			0x20, 0x0F, 0x30, 0x00, 0x00, 0x00, 0x1A}, stmts: 6},
		{inp: []byte{0xFE, 0xD9, 0xA9, 0xBF, 0x54, 0xE2, 0x12, 0xBD, 0x59, 0x20,
			0x65, 0x0D, 0x8F, 0xA2, 0x30, 0x98, 0xD3, 0x3E, 0xD3, 0xF1, 0x06,
			0xE1, 0x44, 0x1C, 0x03, 0xAB, 0x04, 0x8F, 0xED, 0xF0, 0xB3, 0x38,
			0x49, 0x62, 0x1B, 0x60, 0x62, 0x9B, 0x36, 0xF8, 0x1A}, stmts: 5},
		{inp: []byte{0x31, 0x30, 0x20, 0x50, 0x52, 0x49, 0x4E, 0x54, 0x20, 0x22,
			0x54, 0x68, 0x69, 0x73, 0x20, 0x69, 0x73, 0x20, 0x74, 0x68, 0x65, 0x20,
			0x53, 0x74, 0x61, 0x72, 0x74, 0x20, 0x70, 0x72, 0x6F, 0x67, 0x72, 0x61,
			0x6D, 0x2E, 0x22, 0x0A, 0x32, 0x30, 0x20, 0x50, 0x52, 0x49, 0x4E, 0x54,
			0x20, 0x22, 0x53, 0x61, 0x76, 0x65, 0x64, 0x20, 0x61, 0x73, 0x20, 0x41,
			0x53, 0x43, 0x49, 0x49, 0x2E, 0x22}, stmts: 2},
	}

	for _, tt := range tests {
		bts := bytes.NewReader(tt.inp)
		buf := bufio.NewReader(bts)
		var trm mockTerm
		env := object.NewTermEnvironment(trm)

		ParseFile(buf, env)

		if tt.stmts > 0 {
			itr := env.Program.StatementIter()
			assert.Equal(t, tt.stmts, itr.Len(), "Test_ParseFile() expected %d statements but got %d", tt.stmts, itr.Len())
		}
	}
}
