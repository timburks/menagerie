package main

import (
	"bytes"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/apigee/registry/pkg/models"
	yaml "gopkg.in/yaml.v3"
)

var output = "output"

func main() {
	apis := make(map[string]*models.Api)
	filepath.WalkDir("twilio-oai/spec/yaml",
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			} else if entry.IsDir() {
				return nil // Do nothing for the directory, but still walk its contents.
			}
			filename := filepath.Base(path)
			re := regexp.MustCompile(`^twilio_(.*)_(.*)\.yaml$`)
			m := re.FindStringSubmatch(filename)
			if len(m) < 2 {
				return nil
			}
			apiID := strings.ReplaceAll(m[1], "_", "-")
			versionID := m[2]
			bytes, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var node yaml.Node
			if err := yaml.Unmarshal(bytes, &node); err != nil {
				return err
			}
			openapi := yqQueryString(&node, "openapi")
			description := yqQueryString(&node, "info.description")
			if description == nil {
				var empty = ""
				description = &empty
			}
			title := yqQueryString(&node, "info.title")
			if title == nil {
				var empty = ""
				title = &empty
			}
			if strings.HasSuffix(*title, "Api") {
				*title = strings.ReplaceAll(*title, "Api", "API")	
			} else {
				*title += " API"
			}
			*title = strings.ReplaceAll(*title, "Ip_messaging", "IP Messaging")
			*title = strings.ReplaceAll(*title, " - ", " ")
			err = os.MkdirAll(filepath.Join(output, apiID), 0777)
			if err != nil {
				log.Fatalf("%s", err)
			}
			err = os.WriteFile(filepath.Join(output, apiID, filename), bytes, 0666)
			if err != nil {
				log.Fatalf("%s", err)
			}
			api := apis[apiID]
			if api == nil {
				api = &models.Api{
					Header: models.Header{
						ApiVersion: "apigeeregistry/v1",
						Kind:       "API",
						Metadata: models.Metadata{
							Name: "twilio.com-" + apiID,
							Labels: map[string]string{
								"categories": "telecom",
								"provider":   "twilio.com",
							},
						},
					},
					Data: models.ApiData{
						DisplayName: *title,
						Description: *description,
					},
				}
				apis[apiID] = api
			}
			api.Data.ApiVersions = append(api.Data.ApiVersions,
				&models.ApiVersion{
					Header: models.Header{
						Metadata: models.Metadata{
							Name: versionID,
						},
					},
					Data: models.ApiVersionData{
						DisplayName: versionID,
						State:       "production",
						ApiSpecs: []*models.ApiSpec{
							{
								Header: models.Header{
									Metadata: models.Metadata{
										Name: "openapi",
									},
								},
								Data: models.ApiSpecData{
									FileName: filename,
									MimeType: "application/x.openapi+gzip;version=" + *openapi,
								},
							},
						},
					},
				},
			)
			return nil
		})
	for apiID, v := range apis {
		b, err := Encode(v)
		if err != nil {
			return
		}
		err = os.WriteFile(filepath.Join(output, apiID, "info.yaml"), b, 0666)
		if err != nil {
			log.Fatalf("%s", err)
		}
	}
}

// Prefer this encoder because it uses tighter 2-space indentation.
func yamlEncoder(dst io.Writer) *yaml.Encoder {
	enc := yaml.NewEncoder(dst)
	enc.SetIndent(2)
	return enc
}

// Encode a model.
func Encode(model interface{}) ([]byte, error) {
	var b bytes.Buffer
	err := yamlEncoder(&b).Encode(model)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func yqQueryNode(node *yaml.Node, path string) *yaml.Node {
	return query(node, strings.Split(path, "."))
}

func yqQueryString(node *yaml.Node, path string) *string {
	if n := query(node, strings.Split(path, ".")); n == nil {
		return nil
	} else {
		if n.Kind == yaml.ScalarNode {
			return &n.Value
		} else {
			bytes, _ := yaml.Marshal(n)
			s := string(bytes)
			return &s
		}
	}
}

func yqQueryStringArray(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	results := []string{}
	for _, n := range node.Content {
		results = append(results, n.Value)
	}
	return results
}

func query(node *yaml.Node, path []string) *yaml.Node {
	if len(path) == 0 {
		return node
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, c := range node.Content {
			if n := query(c, path); n != nil {
				return n
			}
		}
	case yaml.SequenceNode:
		index, err := strconv.Atoi(path[0])
		if err != nil {
			return nil
		}
		return query(node.Content[index], path[1:])
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == path[0] {
				return query(node.Content[i+1], path[1:])
			}
		}
	case yaml.ScalarNode:
		return node
	case yaml.AliasNode:
		return nil
	default:
		return nil
	}
	return nil
}

func yqDescribe(node *yaml.Node) string {
	bytes, err := yaml.Marshal(node)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}
