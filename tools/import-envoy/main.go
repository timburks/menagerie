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
var provider = "envoyproxy.io"

// List source repos. If a string follows ";", it is the proto path in the repo.
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
		err := describeAPI(api, allProtos)
		if err != nil {
			log.Fatalf("error processing %s: %s", api, err)
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
		parts := strings.Split(d, ";") // the second part is the path to the protos
		d = parts[0]
		target := filepath.Join(dir, filepath.Base(d))
		if exists(target) {
			continue
		}
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

// Find all of the directories matching a version name, these should be imported.
func scanDirectoryForAPIs(start string) ([]string, error) {
	// The pattern of an API version directory.
	versionDirectory := regexp.MustCompile("v.*[1-9]+.*")
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
func describeAPI(container string, allProtos []string) error {
	// First get all of the protos in the specified container.
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
	// Compile the protos and get a list of everything they import.
	all, err := referencedProtos(protos, "")
	if err != nil {
		return err
	}
	// Collect the listed files and put them in a zip archive.
	tempDir, err := os.MkdirTemp("", "proto-collect-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	// (Finding the files is tricky)
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
	contents, err := compress.ZipArchiveOfPath(tempDir, tempDir+"/", true)
	if err != nil {
		return err
	}
	// Get the apiID and versionID for use in Registry YAML.
	localApiDir := strings.TrimPrefix(container, dir+"/"+top+"/")
	apiID := strings.ReplaceAll(filepath.Dir(localApiDir), "/", "-")
	apiID = strings.ReplaceAll(apiID, "_", "-")
	versionID := filepath.Base(localApiDir)
	// Make the directory for the API and version YAML.
	err = os.MkdirAll(filepath.Join(out, apiID, versionID), 0777)
	if err != nil {
		return err
	}
	// Save the zipped protos.
	specfilename := "protos" // strings.TrimPrefix(container, "deps/")
	name := filepath.Join(out, apiID, versionID, strings.ReplaceAll(specfilename, "/", "-")+".zip")
	os.WriteFile(name, contents.Bytes(), 0664)
	// Build and save info.yaml for the version.
	apiVersion := &encoding.ApiVersion{
		Header: encoding.Header{
			ApiVersion: encoding.RegistryV1,
			Kind:       "Version",
			Metadata: encoding.Metadata{
				Parent: "apis/" + provider + "-" + apiID,
				Name:   versionID,
			},
		},
		Data: encoding.ApiVersionData{
			DisplayName: versionID,
			ApiSpecs: []*encoding.ApiSpec{
				{
					Header: encoding.Header{
						Metadata: encoding.Metadata{
							Name: "protos",
							Annotations: map[string]string{
								"path": strings.TrimPrefix(container, "deps/"+top+"/"),
							},
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
	// Build and save info.yaml for the API.
	api := &encoding.Api{
		Header: encoding.Header{
			ApiVersion: encoding.RegistryV1,
			Kind:       "API",
			Metadata: encoding.Metadata{
				Name: provider + "-" + apiID,
				Labels: map[string]string{
					"provider":   strings.ReplaceAll(provider, ".", "-"),
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
	// Done!
	return nil
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
// "root" is the root directory for the compilation.
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

// Guess a good display name for an API.
func displayName(name string) string {
	name = strings.ReplaceAll(name, "_", "-")
	parts := strings.Split(name, "-")

	for i, p := range parts {
		parts[i] = capitalize(p)
	}

	out := strings.Join(parts, " ")
	if !strings.HasSuffix(out, " API") {
		out += " API"
	}
	return out
}

// Try to capitalize a name sensibly.
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
