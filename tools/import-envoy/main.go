package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/apigee/registry/cmd/registry/compress"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var dir = "deps"
var top = "data-plane-api"
var deps = []string{
	"https://github.com/bufbuild/protoc-gen-validate",
	"https://github.com/census-instrumentation/opencensus-proto;src",
	"https://github.com/cncf/udpa",
	"https://github.com/cncf/xds",
	"https://github.com/envoyproxy/data-plane-api",
	"https://github.com/googleapis/googleapis",
	"https://github.com/open-telemetry/opentelemetry-proto",
	"https://github.com/prometheus/client_model",
}
var out = "out"

func main() {

	err := fetchDependencies(dir, deps)
	if err != nil {
		panic(err)
	}

	allProtos, err := listAllProtos(dir)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%d protos\n", len(allProtos))

	apis, err := scanDirectoryForAPIs(filepath.Join(dir, top))
	if err != nil {
		panic(err)
	}
	for _, api := range apis {
		err := compileAPI(api, allProtos)
		if err != nil {
			log.Fatalf("error compiling %s: %s", api, err)
		}
	}
}

func listAllProtos(dir string) ([]string, error) {
	protos := make([]string, 0)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".proto") {
			protos = append(protos, p)
		}
		return nil
	})
	return protos, err

}
func fetchDependencies(dir string, deps []string) error {
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	for _, d := range deps {
		parts := strings.Split(d, ";")
		d = parts[0]
		target := filepath.Join(dir, filepath.Base(d))
		if exists(target) {
			fmt.Printf("reusing %s\n", target)
			continue
		}
		fmt.Printf("fetching %s into %s\n", d, target)
		cmd := exec.Command("git", "clone", d)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("%s", string(out))
			return err
		}
	}
	return nil
}

// exists returns whether the given file or directory exists
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// The pattern of an API version directory.
var versionDirectory = regexp.MustCompile("v.*[1-9]+.*")

func scanDirectoryForAPIs(start string) ([]string, error) {
	apis := make([]string, 0)
	err := filepath.Walk(start, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		container := p
		if !info.IsDir() {
			return nil
		}
		if !versionDirectory.MatchString(container) {
			return nil
		}
		apis = append(apis, container)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return apis, nil
}

func compileAPI(container string, allProtos []string) error {
	fmt.Printf("compile %+v\n", container)

	protos := make([]string, 0)
	err := filepath.Walk(container, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".proto") {
			protos = append(protos, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	//fmt.Printf("protos: %+v\n", protos)
	all, err := referencedProtos(protos, "")
	if err != nil {
		return err
	}

	// Collect the listed files.
	tempDir, err := os.MkdirTemp("", "proto-collect-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	for _, a := range all {
		fmt.Printf("--- %s\n", a)
		for _, p := range allProtos {
			if strings.HasSuffix(p, a) {
				fmt.Printf("---   found at %s\n", p)
				err := copyFile(p, filepath.Join(tempDir, a))
				if err != nil {
					return err
				}
				break
			}
		}

	}
	contents, err := compress.ZipArchiveOfPath(tempDir, tempDir, true)
	if err != nil {
		return err
	}
	err = os.MkdirAll(out, 0777)
	if err != nil {
		return err
	}
	filename := strings.TrimPrefix(container, "deps/")
	name := filepath.Join(out, strings.ReplaceAll(filename, "/", "-")+".zip")
	os.WriteFile(name, contents.Bytes(), 0664)
	return err
}

func copyFile(src, dest string) error {
	dir := filepath.Dir(dest)
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, input, 0644)
}

// Get all the protos that are referenced in the compilation of a list of protos.
func referencedProtos(protos []string, root string) ([]string, error) {
	tempDir, err := os.MkdirTemp("", "proto-import-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	args := []string{"-o", tempDir + "/proto.pb", "--include_imports"}

	imports := []string{}
	for _, d := range deps {
		imports = append(imports, "-I")
		imports = append(imports, strings.ReplaceAll(filepath.Join(dir, filepath.Base(d)), ";", "/"))
	}

	args = append(args, imports...)
	args = append(args, protos...)
	cmd := exec.Command("protoc", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("%s", string(out))
		return nil, fmt.Errorf("failed to compile protos with protoc: %s", err)
	}
	return protosFromFileDescriptorSet(tempDir + "/proto.pb")
}

// Get all the protos listed as dependencies in a file descriptor set.
func protosFromFileDescriptorSet(filename string) ([]string, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	fds := &descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(bytes, fds)
	if err != nil {
		return nil, err
	}
	filenameset := make(map[string]bool)
	for _, file := range fds.File {
		filename = *file.Name
		if !strings.HasPrefix(filename, "google/protobuf/") {
			filenameset[filename] = true
		}
	}
	filenames := make([]string, 0)
	for k := range filenameset {
		filenames = append(filenames, k)
	}
	sort.Strings(filenames)
	return filenames, nil
}
