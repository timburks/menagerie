package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/apigee/registry-experimental/pkg/yamlquery"
	"github.com/apigee/registry/pkg/encoding"
	yaml "gopkg.in/yaml.v3"
)

const DiscoveryURL = "https://discovery.googleapis.com/discovery/v1/apis"

func main() {
	err := readDiscoveryList()
	if err != nil {
		log.Printf("%s", err)
	}
}

func readDiscoveryList() error {
	resp, err := http.Get(DiscoveryURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var doc yaml.Node
	err = yaml.Unmarshal(body, &doc)
	if err != nil {
		return err
	}
	items := yamlquery.QueryNode(&doc, "items")
	for _, node := range items.Content {
		discoveryRestUrl := yamlquery.QueryString(node, "discoveryRestUrl")
		if discoveryRestUrl != nil {
			err := readDoc(*discoveryRestUrl)
			if err != nil {
				log.Printf("%s", err)
			}
		}
	}
	return nil
}

func readDoc(discoveryRestUrl string) error {
	resp, err := http.Get(discoveryRestUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var doc yaml.Node
	err = yaml.Unmarshal(body, &doc)
	if err != nil {
		return err
	}
	node := &doc
	apiID := yamlquery.QueryString(node, "name")
	versionID := yamlquery.QueryString(node, "version")
	title := yamlquery.QueryString(node, "title")
	description := yamlquery.QueryString(node, "description")
	ownerDomain := yamlquery.QueryString(node, "ownerDomain")
	ownerName := yamlquery.QueryString(node, "ownerName")

	*apiID = strings.ReplaceAll(*apiID, "_", "-")
	*versionID = strings.ReplaceAll(*versionID, "_", "-")

	if !strings.HasPrefix(*title, *ownerName) {
		*title = *ownerName + " " + *title
	}

	fmt.Printf("%s/%s\n", *apiID, *versionID)
	api := &encoding.Api{
		Header: encoding.Header{
			ApiVersion: "apigeeregistry/v1",
			Kind:       "API",
			Metadata: encoding.Metadata{
				Name: *ownerDomain + "-" + *apiID,
				Labels: map[string]string{
					"provider": strings.ReplaceAll(*ownerDomain, ".", "-"),
				},
			},
		},
		Data: encoding.ApiData{
			DisplayName: *title,
			Description: *description,
		},
	}
	version := &encoding.ApiVersion{
		Header: encoding.Header{
			ApiVersion: "apigeeregistry/v1",
			Kind:       "Version",
			Metadata: encoding.Metadata{
				Parent: "apis/" + *ownerDomain + "-" + *apiID,
				Name:   *versionID,
			},
		},
		Data: encoding.ApiVersionData{
			DisplayName: *versionID,
		},
	}
	spec := &encoding.ApiSpec{
		Header: encoding.Header{
			ApiVersion: "apigeeregistry/v1",
			Kind:       "Spec",
			Metadata: encoding.Metadata{
				Parent: "apis/" + *ownerDomain + "-" + *apiID + "/versions/" + *versionID,
				Name:   "discovery",
			},
		},
		Data: encoding.ApiSpecData{
			MimeType: "application/x.discovery+gzip",
			FileName: "discovery.json",
		},
	}
	err = os.MkdirAll(filepath.Join(*ownerDomain, *apiID, *versionID, "discovery"), 0777)
	if err != nil {
		return err
	}
	b, err := encoding.EncodeYAML(api)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(*ownerDomain, *apiID, "info.yaml"), b, 0666)
	if err != nil {
		return err
	}
	b, err = encoding.EncodeYAML(version)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(*ownerDomain, *apiID, *versionID, "info.yaml"), b, 0666)
	if err != nil {
		return err
	}
	b, err = encoding.EncodeYAML(spec)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(*ownerDomain, *apiID, *versionID, "discovery", "info.yaml"), b, 0666)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(*ownerDomain, *apiID, *versionID, "discovery", "discovery.json"), body, 0666)
	if err != nil {
		return err
	}
	return nil
}
