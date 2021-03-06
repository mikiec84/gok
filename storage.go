package main

import (
  "fmt"
	"github.com/boltdb/bolt"
	"log"
	"github.com/blevesearch/bleve"
  /*"reflect"*/
  "errors"
)

type Storage struct {
	Path  string
	DB    *bolt.DB
	Index bleve.Index
}

var (
	db *bolt.DB
)

func NewStorage(path string) (Storage, error) {
	s := Storage{Path: path}
	db, err := bolt.Open(s.Path+".bolt", 0600, nil)
	if err != nil {
		return s, err
	}
	s.DB = db
	//defer db.Close()

	index, err := bleve.Open(s.Path + ".bleve")
	if err == bleve.ErrorIndexPathDoesNotExist {
		log.Printf("Creating new index...")
		// create a mapping
		indexMapping, err := buildIndexMapping()
		if err != nil {
			log.Fatal(err)
		}
		index, err = bleve.New(s.Path+".bleve", indexMapping)
		if err != nil {
			log.Fatal(err)
		}
	}

	s.Index = index

	return s, nil
}

func (s Storage) Add(item *Item) bool {
	s.DB.Update(func(tx *bolt.Tx) error {

		b, err := tx.CreateBucketIfNotExists([]byte("MyList"))
		if err != nil {
			return err
		}
		result := b.Put([]byte(item.Url), []byte(item.Title))
		return result
	})

	s.Index.Index(item.Url, item)

	return true
}

func (s Storage) findByUrl(url string) (*Item, error) {
  item := &Item{url, "", ""}
  s.DB.View(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("MyList"))
    v := b.Get([]byte(url))
    item.Title = string(v)
    return nil
  })
  if item.Title == "" {
    return nil, errors.New("Not found")
  }
  return item, nil
}

func (s Storage) List() ([]*Item, error) {
  //We only return 100 items at a time
  result := make([]*Item, 0, 100)

	err := s.DB.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte("MyList"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			/*fmt.Printf("|%s | %s\n|", k, v)*/
      result = append(result, &Item{string(k), string(v), ""})
		}

    return nil

	})

	if err != nil {
		//log.Fatal(err)
    return nil, err
	}

  return result, nil
}

func (s Storage) Search(url string) ([]*Item, error) {
	// search for some text
	query := bleve.NewMatchQuery(url)
	search := bleve.NewSearchRequest(query)
	searchResults, err := s.Index.Search(search)
	if err != nil {
		return nil, err
	}

  result := make([]*Item, 0, 100)
  if searchResults.Total == 0 {
    return result, nil
  }

  for index, document := range searchResults.Hits {
    result = append(result, &Item{"", "", ""})
    result[index], _ = s.findByUrl(document.ID)

    for field, value := range document.Fragments {
      fmt.Println("sasa %s", document)
      fmt.Println("%d.... %s %s", index, field, value)
      /*f := reflect.ValueOf(&result[index]).Elem().FieldByName(fmt.Sprintf("%s", field)).SetString(value)*/
    }
  }

  /*fmt.Println(searchResults)*/
  return result, nil
}

func buildIndexMapping() (*bleve.IndexMapping, error) {
	englishTextFieldMapping := bleve.NewTextFieldMapping()
	englishTextFieldMapping.Analyzer = "en"

	// a generic reusable mapping for keyword text
	keywordFieldMapping := bleve.NewTextFieldMapping()
	keywordFieldMapping.Analyzer = "keyword"

	urlMapping := bleve.NewDocumentMapping()

	urlMapping.AddFieldMappingsAt("url", keywordFieldMapping)

	urlMapping.AddFieldMappingsAt("title", englishTextFieldMapping)
	urlMapping.AddFieldMappingsAt("body", keywordFieldMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddDocumentMapping("url", urlMapping)
	indexMapping.TypeField = "type"
	indexMapping.DefaultAnalyzer = "en"
	return indexMapping, nil
}
