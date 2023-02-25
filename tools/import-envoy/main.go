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
	"github.com/apigee/registry/pkg/encoding"
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
var out = "envoyproxy.io"

func main() {
	err := fetchDependencies(dir, deps)
	if err != nil {
		panic(err)
	}
	allProtos, err := listAllProtos(dir)
	if err != nil {
		panic(err)
	}
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

// Build a list of all proto files in a directory.
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

// Use git clone to fetch a list of repo dependencies.
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
			//fmt.Printf("reusing %s\n", target)
			continue
		}
		//fmt.Printf("fetching %s into %s\n", d, target)
		cmd := exec.Command("git", "clone", d)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("%s", string(out))
			return err
		}
	}
	return nil
}

// Return true if a file exists.
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
		return filepath.SkipDir
	})
	if err != nil {
		return nil, err
	}
	return apis, nil
}

// Compile an API description and create Registry YAML.
func compileAPI(container string, allProtos []string) error {
	//fmt.Printf("compile %+v\n", container)
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
		for _, p := range allProtos {
			if strings.HasSuffix(p, a) {
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
	// Save the API
	apiDir := strings.TrimPrefix(container, dir+"/"+top+"/")
	versionID := filepath.Base(apiDir)
	apiID := strings.ReplaceAll(filepath.Dir(apiDir), "/", "-")
	apiID = strings.ReplaceAll(apiID, "_", "-")
	err = os.MkdirAll(filepath.Join(out, apiID, versionID), 0777)
	if err != nil {
		return err
	}
	specfilename := "protos" // strings.TrimPrefix(container, "deps/")
	name := filepath.Join(out, apiID, versionID, strings.ReplaceAll(specfilename, "/", "-")+".zip")
	os.WriteFile(name, contents.Bytes(), 0664)

	apiVersion := &encoding.ApiVersion{
		Header: encoding.Header{
			ApiVersion: encoding.RegistryV1,
			Kind:       "Version",
			Metadata: encoding.Metadata{
				Parent: "apis/envoyproxy.io-" + apiID,
				Name:   versionID,
			},
		},
		Data: encoding.ApiVersionData{
			ApiSpecs: []*encoding.ApiSpec{
				{
					Header: encoding.Header{
						Metadata: encoding.Metadata{
							Name: "protos",
						},
					},
					Data: encoding.ApiSpecData{
						FileName: "protos.zip",
						MimeType: "application/x.proto+zip",
					},
				},
			},
		},
	}
	b, err := encoding.EncodeYAML(apiVersion)
	if err != nil {
		return err
	}
	name = filepath.Join(out, apiID, versionID, "info.yaml")
	err = os.WriteFile(name, b, 0664)
	if err != nil {
		return err
	}

	api := &encoding.Api{
		Header: encoding.Header{
			ApiVersion: encoding.RegistryV1,
			Kind:       "API",
			Metadata: encoding.Metadata{
				Name: "envoyproxy.io-" + apiID,
				Labels: map[string]string{
					"provider":   "envoyproxy-io",
					"categories": "networking",
				},
			},
		},
		Data: encoding.ApiData{
			DisplayName: displayName(apiID),
		},
	}
	b, err = encoding.EncodeYAML(api)
	if err != nil {
		return err
	}
	name = filepath.Join(out, apiID, "info.yaml")
	err = os.WriteFile(name, b, 0664)
	if err != nil {
		return err
	}

	return err
}

// Copy a file from one path to another, ensuring that the destination directory exists.
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

func displayName(name string) string {
	name = strings.ReplaceAll(name, "_", "-")
	parts := strings.Split(name, "-")

	for i, p := range parts {
		parts[i] = capitalize(p)
	}

	out := strings.Join(parts, " ") + " API"
	fmt.Printf("%s\n", out)
	return out
}

func capitalize(s string) string {
	switch s {
	case "to":
		return s
	case "grpc":
		return "gRPC"
	case "gzip":
		return "GZip"
	case "oauth":
		return "OAuth"
	case "oauth2":
		return "OAuth2"
	case "mysql":
		return "MySQL"
	case "rocketmq":
		return "RocketMQ"
	case "api", "aws", "cors", "csrf", "dlb", "dns", "dst", "gcp", "http", "http1",
		"ip", "jwt", "qat", "rbac", "s2a", "sip", "sni", "src",
		"ssl", "sxg", "tcp", "tls", "tra", "udp", "vcl", "wasm":
		return strings.ToTitle(s)
	}
	return strings.Title(s)
}
