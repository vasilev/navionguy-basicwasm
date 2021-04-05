package fileserv

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"unicode"

	"github.com/gorilla/mux"
	"github.com/navionguy/basicwasm/filelist"
	"github.com/navionguy/basicwasm/object"
)

// The first group of functions are used by the basicwasm server.
// To serve files down to the interpreter running in the browser.
// The second group are used to build requests and to process
// the results of those requests.

type fileSource struct {
	src      http.FileSystem
	filename string
}

// These are the command line flags that tell where to find runtime resources
var (
	assetsDir = flag.String("assets", "./assets/", "web page assets")
	moduleDir = flag.String("webmodules", "./webmodules/", "web assembly file(s)")
	drives    = map[string]*string{
		"driveA": flag.String("driveA", "", ""),
		"driveB": flag.String("driveB", "", ""),
		"driveC": flag.String("driveC", "./source", "current directory on start-up"),
		// TODO: add the rest of the possible drive letter flags
	}
)

// WrapFileSources builds mux routes to all my resources
// css files, images, javascript files and of course
// the basic interpreter wasm file.
// Then he maps all the drive letters that point to a
// file store.
//
// ToDo: drive the resource mapping from a table
func WrapFileSources(rtr *mux.Router) {
	resources := []struct {
		rootdir  string
		subdir   string
		route    string
		mimetype string
	}{
		{rootdir: *assetsDir, subdir: "css/", route: "/css/{file}.{ext}", mimetype: "text/css"},
		{rootdir: *assetsDir, subdir: "images/", route: "/images/{file}.{ext}", mimetype: "text/plain"},
		{rootdir: *assetsDir, subdir: "js/", route: "/js/{file}.{ext}", mimetype: "application/x-javascript; charset=utf-8"},
		{rootdir: *moduleDir, route: "/wasm/{file}.{ext}", mimetype: "application/wasm"},
	}

	for _, res := range resources {
		drv := res.rootdir + res.subdir
		fs := &fileSource{src: http.Dir(drv)}
		fs.wrapSource(rtr, res.route, res.mimetype)
	}

	for key, drv := range drives {
		if len(*drv) > 0 {
			fs := &fileSource{src: http.Dir(*drv)}
			path := "/" + key
			fs.fullyWrapSource(rtr, path)
			fs.wrapSubDirs(rtr, *drv, path)
		}
	}
}

// given a path, create a handler function that will extract the
// parts of the path and then call the source directory to work
// on the file
func (fs *fileSource) wrapSource(rtr *mux.Router, path string, mimetype string) {
	rtr.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		vs := mux.Vars(r)
		file := vs["file"]
		ext := vs["ext"]

		if len(ext) > 0 {
			file = file + "." + ext
		}
		fs.serveFile(rw, r, file, mimetype)
	}).Name(path)

}

// Since the gorilla mux doesn't support wildcard routes I have to map
// all the possibilities independantly.
// 		http://hostname:port/driveC
// 		http://hostname:port/driveC/
// 		http://hostname:port/driveC/program
// 		http://hostname:port/driveC/program.ext
func (fs *fileSource) fullyWrapSource(rtr *mux.Router, path string) {
	fs.wrapSource(rtr, path, "text/plain; charset=ASCII")
	fs.wrapSource(rtr, path+"/", "text/plain; charset=ASCII")
	fs.wrapSource(rtr, path+"/{file}.{ext}", "text/plain; charset=ASCII")
	fs.wrapSource(rtr, path+"/{file}", "text/plain; charset=ASCII")
}

// After wrapping a directory, I want to wrap any sub-directories
// he might have.
//
func (fs *fileSource) wrapSubDirs(rtr *mux.Router, dir string, path string) {
	hfile, err := fs.src.Open("/")

	// if I can't open him, nothing more to do
	if err != nil {
		return
	}
	defer hfile.Close()

	files, err := hfile.Readdir(-1)

	// he might not be a directory
	if err != nil {
		return
	}

	// he is a directory
	// go wrap any sub directories
	fs.wrapADir(rtr, dir, path, files)

}

// loops through filenames looking for directories
// wraps the directories and then calls wrapSubDirs on them
// to understand recursion, you must understand recursion
//
func (fs fileSource) wrapADir(rtr *mux.Router, dir string, path string, files []os.FileInfo) {
	for _, finfo := range files {
		if containsDotFile(finfo.Name()) {
			continue
		}

		tFile, err := fs.src.Open(finfo.Name())

		if err != nil {
			continue
		}

		info, err := tFile.Stat()

		if err != nil {
			continue
		}
		tFile.Close()

		if !info.IsDir() {
			continue
		}

		fname := info.Name()
		subdir := dir + "/" + fname
		subpath := path + "/" + fname
		nfs := &fileSource{src: http.Dir(subdir)}
		nfs.fullyWrapSource(rtr, subpath)
		nfs.wrapSubDirs(rtr, subdir, subpath)
	}
}

// serveFile opens up the file and sends its contents
//
func (fs fileSource) serveFile(w http.ResponseWriter, r *http.Request, fname string, mimetype string) {
	if len(fname) == 0 {
		fname = "/"
	}

	hfile, err := fs.Open(fname)

	if err != nil {
		w.WriteHeader(404)
		return
	}

	st, err := hfile.Stat()

	if err != nil {
		w.WriteHeader(500)
		return
	}

	if st.IsDir() {
		fs.sendDirectory(hfile, w)
		return
	}

	buf := make([]byte, int(st.Size()))
	_, err = hfile.Read(buf)

	if err != nil {
		w.WriteHeader(503)
		return
	}

	if len(mimetype) > 0 {
		w.Header().Set("Content-Type", mimetype)
	}
	w.Write(buf)

}

// sendDirectory sends all the filenames found in hfile
// he does block any that start with '.'
func (fs fileSource) sendDirectory(hfile http.File, w http.ResponseWriter) {
	files, err := hfile.Readdir(-1)

	if err != nil {
		w.WriteHeader(404)
		return
	}

	fl := filelist.NewFileList()
	for _, finfo := range files {
		if !containsDotFile(finfo.Name()) {
			fl.AddFile(finfo)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(fl.JSON())
}

// Open is a wrapper around the Open method of the embedded FileSystem
// that builds the actual file name based on his extension and how
// my assets are arranged.
func (fs fileSource) Open(name string) (hFile http.File, err error) {
	if containsDotFile(name) { // If dot file, return 403 response
		return nil, os.ErrPermission
	}

	file, err := fs.src.Open(name)
	if err != nil {
		return nil, err
	}

	return dotFileHidingFile{file}, nil

}

// containsDotFile reports whether name contains a path element starting with a period.
// The name is assumed to be a delimited by forward slashes, as guaranteed
// by the http.FileSystem interface.
func containsDotFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// dotFileHidingFile is the http.File use in dotFileHidingFileSystem.
// It is used to wrap the Readdirnames method of http.File so that we can
// remove files and directories that start with a period from its output.
type dotFileHidingFile struct {
	http.File
}

// Readdir is a wrapper around the Readdir method of the embedded File
// that filters out all files that start with a period in their name.
func (f dotFileHidingFile) Readdir(n int) (fis []os.FileInfo, err error) {
	files, err := f.File.Readdir(n)
	for _, file := range files { // Filters out the dot files
		if !strings.HasPrefix(file.Name(), ".") {
			fis = append(fis, file)
		}
	}
	return
}

// Functions below here are used in the interpreter to request
// files from the file handlers defined above

// GetFile fetches
func GetFile(file string, env *object.Environment) (*[]byte, error) {
	rq := buildRequestURL(file, env)
	res, err := sendRequest(rq, env)

	if err != nil {
		return nil, err
	}

	body, _ := ioutil.ReadAll(res.Body)

	return &body, nil
}

func sendRequest(rq string, env *object.Environment) (*http.Response, error) {
	res, err := env.GetClient().Get(rq)

	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, errors.New("File not found")
	}

	return res, nil
}

// build up a URL for addressing the target file
func buildRequestURL(target string, env *object.Environment) string {
	url := getURL(env)
	cwd := getCWD(env)
	target = convertDrive(target, cwd)

	return url + target
}

// Get the URL of my server, he hides it in the HTML
// and main pushes it into the environment object store
func getURL(env *object.Environment) string {

	mom, ok := env.Get(object.SERVER_URL)
	if !ok {
		mom = &object.String{Value: "http://localhost:8080/"}
	}
	url := mom.Inspect()
	if url[len(url)-1:] != "/" {
		url = url + "/"
	}

	return url
}

// Get the current working directory from the environment
func getCWD(env *object.Environment) string {
	drv, ok := env.Get(object.WORK_DRIVE)
	if !ok { // if he wasn't set, use a default
		drv = &object.String{Value: `C:\`}
	}

	return drv.Inspect()
}

// convert from:
//		C:\DIRNAME\FILENAME.EXT
// to
//		driveC/DIRNAME/FILENAME.EXT
func convertDrive(target, cwd string) string {
	if len(target) == 0 {
		cwd = strings.ReplaceAll(cwd, `\`, "/")
		drv := "drive" + strings.ToUpper(cwd[0:1])
		return drv
	}

	target = strings.ReplaceAll(target, `\`, "/")
	if checkForDrive(target) {
		// if he starts with a drive
		// we can ignore current working directory
		drv := "drive" + strings.ToUpper(target[0:1])
		return path.Join(drv, target[2:])
	}

	if `/` == target[0:1] {
		// he wants to start at root of current drive
		drv := "drive" + strings.ToUpper(cwd[0:1])
		return path.Join(drv, target)
	}

	// start from cwd
	cwd = strings.ReplaceAll(cwd, `\`, "/")
	drv := "drive" + strings.ToUpper(cwd[0:1])
	return path.Join(drv, cwd[2:], target)
}

// check to see if a drive is specified
func checkForDrive(path string) bool {
	// less than 2 char can't be a drive spec
	if len(path) < 2 {
		//
		return false
	}

	// letter followed by colon, it's a drive spec
	if (unicode.IsLetter(rune(path[0:1][0]))) && (":" == path[1:2]) {
		return true
	}

	return false
}

// FormatFileName forces it into 8.3 form
func FormatFileName(name string, isDir bool) string {
	// split off any extension so I can catch long basenames
	prts := strings.Split(name, ".")
	if len(prts) == 1 {
		prts = append(prts, " ") // blank extension
	}

	output := fmt.Sprintf("%-8.8s.%-3.3s%s", formatBaseName(prts[0]), formatExtension(prts[1]), setDirTag(isDir))
	return output
}

// if the basename is longer than eight characters
// GWBasic would show the first seven with a '+'
// in the eighth position
func formatBaseName(name string) string {
	if len(name) > 8 {
		name = name[:7] + "+"
	}

	return name
}

// The 8.3 file format was just assumed at the time
// Now, extensions can be as long as you want
// you can I have multiple parts to a name
// I just use the part after the first period
// and trim to three characters if needed
func formatExtension(ext string) string {
	if len(ext) > 3 {
		ext = ext[:3]
	}
	return ext
}

func setDirTag(isDir bool) string {
	if isDir {
		return "<dir>"
	}

	return "    "
}
