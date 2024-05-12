package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const (
	API_ROOT = "/api/v1"
)

type ChromaClient struct {
	baseURL    *url.URL
	tenant     string
	database   string
	httpClient *http.Client
}

type ChromaTenant struct {
	Name string `json:"name"`
}

type ChromaDatabase struct {
	Name   string `json:"name"`
	Tenant string `json:"tenant,omitempty"`
	ID     string `json:"id,omitempty"`
}

type ChromaCollection struct {
	Name     string        `json:"name,omitempty"`
	ID       string        `json:"id,omitempty"`
	Tenant   string        `json:"tenant,omitempty"`
	Database string        `json:"database,omitempty"`
	client   *ChromaClient `json:"-"` // keeps reference on Client
	// Metadata field allows to customize the distance method https://docs.trychroma.com/usage-guide#changing-the-distance-function
}

type ChromaCollectionEntry struct {
	Embedding *[]float64     `json:"embedding,omitempty"`
	Document  string         `json:"document,omitempty"`
	Metadatas map[string]any `json:"metadatas,omitempty"`
	ID        string         `json:"id,omitempty"`
	Distance  float64        `json:"distance,omitempty"`
}

type ChromaCollectionEntries struct {
	Embeddings []*[]float64     `json:"embeddings,omitempty"`
	Documents  []string         `json:"documents,omitempty"`
	Metadatas  []map[string]any `json:"metadatas,omitempty"`
	IDs        []string         `json:"ids,omitempty"`
	Distances  []float64        `json:"distances,omitempty"`
	// uris
	// data
}

type ChromaCollectionBatchEntries struct {
	Embeddings [][][]float64      `json:"embeddings,omitempty"`
	Documents  [][]string         `json:"documents,omitempty"`
	Metadatas  [][]map[string]any `json:"metadatas,omitempty"`
	IDs        [][]string         `json:"ids,omitempty"`
	Distances  [][]float64        `json:"distances,omitempty"`
	// uris
	// data
}

type ChromaCollectionQuery struct {
	Embeddings [][]float64 `json:"query_embeddings,omitempty"`
	// Texts []string `json:"query_texts,omitempty"` // not supported yet
	NResults      int            `json:"n_results,omitempty"`
	Where         *Where         `json:"where,omitempty"`
	WhereDocument *WhereDocument `json:"where_document,omitempty"`
}

func NewChromaClient(baseURLStr, tenant, database string) (*ChromaClient, error) {

	// Base URL
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return nil, errors.New("Error parsing base URL:" + err.Error())
	}

	baseURL.Path = path.Join(baseURL.Path, API_ROOT)

	if DEBUG {
		fmt.Println("CLIENT URL:", baseURL.String())
	}

	return &ChromaClient{
		baseURL:    baseURL,
		tenant:     tenant,
		database:   database,
		httpClient: &http.Client{},
	}, nil
}

func (c *ChromaClient) Check() error {

	tenant, err := c.GetTenant()
	if err != nil {
		return err
	}
	if tenant == nil {
		return errors.New("tenant is nil")
	}
	if DEBUG {
		fmt.Println("TENANT:")
		printStruct(tenant)
	}

	database, err := c.GetDatabase()
	if err != nil {
		return err
	}
	if database == nil {
		return errors.New("database is nil")
	}
	if DEBUG {
		fmt.Println("DATABASE:")
		printStruct(database)
	}

	testCollection, err := c.GetCollection("test_collection")
	if err != nil {
		return err
	}
	if testCollection == nil {
		return errors.New("collection is nil")
	}
	if DEBUG {
		fmt.Println("COLLECTION:")
		printStruct(testCollection)
	}

	err = testCollection.Add([]ChromaCollectionEntry{
		{
			Embedding: &[]float64{1.0, 1.0, 1.0},
			Document:  "test2",
			Metadatas: map[string]any{"createdAt": 1234},
			ID:        "baz",
		},
	})

	if err != nil {
		return err
	}

	results, err := testCollection.Query(ChromaCollectionQuery{
		Embeddings: [][]float64{
			{1.0, 1.0, 0.999},
		},
	})

	if err != nil {
		return err
	}
	if results == nil {
		return errors.New("results is nil")
	}
	if DEBUG {
		fmt.Println("RESULTS:")
		printStruct(results)
	}

	err = c.RemoveCollection("test_collection")
	if err != nil {
		return err
	}

	return nil
}

// Gets tenant, creating it if not found
func (c *ChromaClient) GetTenant() (*ChromaTenant, error) {
	url := *c.baseURL

	url.Path = path.Join(url.Path, "tenants", c.tenant)
	// fmt.Println("GetTenant URL:", url.String())

	resp, err := c.httpClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		// status code not super reliable...  checking content of returned message too.
		// (got a 500 with a "not found" error message)
		// {"error":"NotFoundError('Tenant new_tenant not found')"}
		if resp.StatusCode == http.StatusNotFound || strings.Contains(string(body), "not found") {
			url := *c.baseURL
			url.Path = path.Join(url.Path, "tenants")
			// fmt.Println("URL:", url.String())

			tenant := ChromaTenant{Name: c.tenant}

			payload, err := json.Marshal(tenant)
			if err != nil {
				return nil, err
			}

			// fmt.Println("payload:", string(payload))

			resp, err := c.httpClient.Post(url.String(), "application/json", bytes.NewBuffer(payload))
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				// fmt.Printf("STATUS: %d\n", resp.StatusCode)
				// body, _ := ioutil.ReadAll(resp.Body)
				// fmt.Println("BODY:", string(body))
				return nil, errors.New("failed to create tenant")
			}

			return &tenant, nil
		}

		return nil, errors.New("non-supported HTTP status:" + strconv.Itoa(resp.StatusCode))
	}

	decoder := json.NewDecoder(resp.Body)
	var tenant ChromaTenant
	err = decoder.Decode(&tenant)
	if err != nil {
		return nil, err
	}

	return &tenant, nil
}

// Gets database, creating it if not found
func (c *ChromaClient) GetDatabase() (*ChromaDatabase, error) {
	url := *c.baseURL

	url.Path = path.Join(url.Path, "databases", c.database)
	query := url.Query()
	query.Set("tenant", c.tenant)
	url.RawQuery = query.Encode()
	// fmt.Println("GetDatabase URL:", url.String())

	resp, err := c.httpClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		// status code not super reliable...  checking content of returned message too.
		// (got a 500 with a "not found" error message)
		if resp.StatusCode == http.StatusNotFound || strings.Contains(string(body), "not found") {
			url := *c.baseURL
			url.Path = path.Join(url.Path, "databases")
			query := url.Query()
			query.Set("tenant", c.tenant)
			url.RawQuery = query.Encode()
			// fmt.Println(">> URL:", url.String())

			database := ChromaDatabase{Name: c.database}

			payload, err := json.Marshal(database)
			if err != nil {
				return nil, err
			}

			// fmt.Println("payload:", string(payload))

			resp, err := c.httpClient.Post(url.String(), "application/json", bytes.NewBuffer(payload))
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				// fmt.Printf("STATUS: %d\n", resp.StatusCode)
				// body, _ := ioutil.ReadAll(resp.Body)
				// fmt.Println("BODY:", string(body))
				return nil, errors.New("failed to create database")
			}

			return &database, nil
		}

		return nil, errors.New("non-supported HTTP status:" + strconv.Itoa(resp.StatusCode))
	}

	decoder := json.NewDecoder(resp.Body)
	var database ChromaDatabase
	err = decoder.Decode(&database)
	if err != nil {
		return nil, err
	}

	return &database, nil
}

// Removes collection
func (c *ChromaClient) RemoveCollection(name string) error {
	url := *c.baseURL

	url.Path = path.Join(url.Path, "collections", name)
	query := url.Query()
	query.Set("tenant", c.tenant)
	query.Set("database", c.database)
	url.RawQuery = query.Encode()

	fmt.Println("RemoveCollection:", url.String())

	req, err := http.NewRequest("DELETE", url.String(), nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("RemoveCollection: wrong HTTP status:" + strconv.Itoa(resp.StatusCode))
	}

	return nil
}

// Gets collection, creating it if not found
func (c *ChromaClient) GetCollection(name string) (*ChromaCollection, error) {
	url := *c.baseURL

	url.Path = path.Join(url.Path, "collections", name)
	query := url.Query()
	query.Set("tenant", c.tenant)
	query.Set("database", c.database)
	url.RawQuery = query.Encode()
	// fmt.Println("GetCollection URL:", url.String())

	resp, err := c.httpClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		// status code not super reliable...  checking content of returned message too.
		// (got a 500 with a "not found" error message)
		// {"error":"ValueError('Collection npcs does not exist.')"}
		if resp.StatusCode == http.StatusNotFound || strings.Contains(string(body), "does not exist") {
			url := *c.baseURL
			url.Path = path.Join(url.Path, "collections")
			query := url.Query()
			query.Set("tenant", c.tenant)
			query.Set("database", c.database)
			url.RawQuery = query.Encode()
			fmt.Println(">> URL:", url.String())

			collection := ChromaCollection{Name: name}

			payload, err := json.Marshal(collection)
			if err != nil {
				return nil, err
			}

			// fmt.Println("payload:", string(payload))

			resp, err := c.httpClient.Post(url.String(), "application/json", bytes.NewBuffer(payload))
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				// fmt.Printf("STATUS: %d\n", resp.StatusCode)
				return nil, errors.New("failed to create database")
			}

			decoder := json.NewDecoder(resp.Body)
			var createdCollection ChromaCollection
			err = decoder.Decode(&createdCollection)
			if err != nil {
				return nil, err
			}

			createdCollection.client = c

			return &createdCollection, nil
		}

		return nil, errors.New("non-supported HTTP status:" + strconv.Itoa(resp.StatusCode))
	}

	decoder := json.NewDecoder(resp.Body)
	var collection ChromaCollection
	err = decoder.Decode(&collection)
	if err != nil {
		return nil, err
	}

	collection.client = c

	return &collection, nil
}

func (c *ChromaCollection) Add(entries []ChromaCollectionEntry) error {

	len := len(entries)

	collectionEntries := &ChromaCollectionEntries{
		Embeddings: make([]*[]float64, len),
		Documents:  make([]string, len),
		Metadatas:  make([]map[string]any, len),
		IDs:        make([]string, len),
	}

	for i, entry := range entries {
		collectionEntries.Embeddings[i] = entry.Embedding
		collectionEntries.Documents[i] = entry.Document
		collectionEntries.Metadatas[i] = entry.Metadatas
		collectionEntries.IDs[i] = entry.ID
	}

	url := *c.client.baseURL
	url.Path = path.Join(url.Path, "collections", c.ID, "add")

	payload, err := json.Marshal(collectionEntries)
	if err != nil {
		return err
	}

	// fmt.Println("collection.add payload:", string(payload))

	resp, err := c.client.httpClient.Post(url.String(), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return errors.New("HTTP status:" + strconv.Itoa(resp.StatusCode))
	}

	return nil
}

func (c *ChromaCollection) Query(query ChromaCollectionQuery) ([]ChromaCollectionEntry, error) {

	url := *c.client.baseURL
	url.Path = path.Join(url.Path, "collections", c.ID, "query")

	payload, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	// fmt.Println("collection.query payload:", string(payload))

	resp, err := c.client.httpClient.Post(url.String(), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// body, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return nil, err
	// }
	// fmt.Println("body:", string(body))

	// {"ids":[["","foo"]],"distances":[[1.0000000000000019e-6,1.0000000000000019e-6]],"metadatas":[[null,{"createdAt":1234}]],"embeddings":null,"documents":[["","test"]],"uris":null,"data":null}

	decoder := json.NewDecoder(resp.Body)
	var entries ChromaCollectionBatchEntries
	err = decoder.Decode(&entries)
	if err != nil {
		return nil, err
	}

	if len(entries.IDs) < 1 {
		return nil, errors.New("there should be at least one entry")
	}

	ids := entries.IDs[0]
	nbEntries := len(ids)

	results := make([]ChromaCollectionEntry, nbEntries)
	for i := 0; i < nbEntries; i++ {
		results[i] = ChromaCollectionEntry{
			Embedding: nil,
			ID:        entries.IDs[0][i],
			Document:  entries.Documents[0][i],
			Distance:  entries.Distances[0][i],
			Metadatas: entries.Metadatas[0][i],
		}
	}

	return results, nil
}

func printStruct(v any) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(jsonBytes))
}
