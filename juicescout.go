package main

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"

	"os"
	"path"
	"strconv"
	"strings"

	"github.com/antonholmquist/jason"
	"github.com/codegangsta/cli"
	"github.com/joho/godotenv"
	"github.com/parnurzeal/gorequest"
)

var categoriesPath, scoutAPI, collectionID string

// Category is imported from HelpJuice's exported CSV
type Category struct {
	id     int    // HelpJuice Category ID
	parent int    // HelpJuice Category Parent ID
	name   string // HelpJuice Category Name
}

// CategoryMapping matches a HelpScout Category to a HelpJuice Category
type CategoryMapping struct {
	juiceID int    // HelpJuice Category ID
	scoutID string // HelpScout Category ID
	name    string // Category name
}

func errorCheck(e error) {
	if e != nil {
		panic(e)
	}
}

func errsCheck(errs []error) {
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Println(err)
		}
		log.Fatal("Crashing...")
	}
}

func checkAPIError(APIError string) {
	parsed, err := jason.NewObjectFromBytes([]byte(APIError))
	errorCheck(err)
	parsedName, err := parsed.GetStringArray("name")
	errorCheck(err)
	fmt.Println(parsedName[0])
}

// Parse the provided categories CSV
func parseCategories() [][]string {
	filePath := path.Clean(categoriesPath)
	f, err := ioutil.ReadFile(filePath)
	errorCheck(err)

	r := csv.NewReader(strings.NewReader(string(f)))

	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Succesfully parsed: ", filePath)
	return records
}

func processCategories(categories [][]string) []Category {
	processedCategories := []Category{}
	for index, category := range categories {
		// Skip the header column
		if index == 0 {
			continue
		}
		id, _ := strconv.Atoi(category[0])
		parent, _ := strconv.Atoi(category[1])
		name := category[2]
		processedCategories = append(processedCategories, Category{id, parent, name})
		fmt.Println("Processed category: ", name)
	}

	return processedCategories
}

func migrateCategories(categories []Category) []CategoryMapping {
	request := gorequest.New().SetBasicAuth(scoutAPI, "X")

	// Get collections to find collection id
	_, collectionsBody, errs := request.
		Get("https://docsapi.helpscout.net/v1/collections").
		End()
	errsCheck(errs)

	collectionsParsed, err := jason.NewObjectFromBytes([]byte(collectionsBody))
	errorCheck(err)

	items, err := collectionsParsed.GetObjectArray("collections", "items")
	errorCheck(err)

	// Just grab the ID of the first one
	collectionID, err := items[0].GetString("id")
	errorCheck(err)

	// Create the categories
	for _, category := range categories {
		categoryObject := `{"collectionId": "` + collectionID + `", "name": "` + category.name + `"}`
		resp, categoryBody, errs := request.
			Post("https://docsapi.helpscout.net/v1/categories").
			Send(categoryObject).
			End()
		errsCheck(errs)

		if resp.StatusCode != 201 {
			checkAPIError(categoryBody)
		} else {
			fmt.Println("Success! ", category.name, " has been created on HelpScout.")
		}
	}

	// Link HelpScout and HelpJuice category IDs
	categoryListReq, categoryListBody, errs := request.
		Get("https://docsapi.helpscout.net/v1/collections/" + collectionID + "/categories").
		End()
	errsCheck(errs)

	var categoryMappings []CategoryMapping
	if categoryListReq.StatusCode == 200 {
		categoryListParsed, err := jason.NewObjectFromBytes([]byte(categoryListBody))
		errorCheck(err)

		categoryListItems, err := categoryListParsed.GetObjectArray("categories", "items")
		for _, listItem := range categoryListItems {
			scoutID, err := listItem.GetString("id")
			errorCheck(err)
			name, err := listItem.GetString("name")
			errorCheck(err)

			// Yes, for each in a for each, but apparently this is the way to do it in Go
			var juiceID int
			for _, c := range categories {
				if c.name == name {
					juiceID = c.id
				}
			}
			categoryMappings = append(categoryMappings, CategoryMapping{juiceID, scoutID, name})
		}
	} else {
		checkAPIError(categoryListBody)
	}

	return categoryMappings
}

func migrateQuestions(categoryMappings []CategoryMapping) {
	fmt.Println(categoryMappings)
}

func main() {
	// Initialize dotenv
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize app
	app := cli.NewApp()
	app.Name = "JuiceScout"
	app.Usage = "Migrate HelpJuice docs over to HelpScout"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "categoriesPath, c",
			Usage:       "Path of the HelpJuice categories.csv file",
			Destination: &categoriesPath,
		},
		cli.StringFlag{
			Name:        "scoutAPI, s",
			Usage:       "API key for HelpScout",
			EnvVar:      "HELPSCOUT_API",
			Destination: &scoutAPI,
		},
	}

	app.Action = func(c *cli.Context) {
		// Required flag warning
		if categoriesPath == "" {
			log.Fatalln("categoriesPath flag is required")
		}

		categories := parseCategories()
		processedCategories := processCategories(categories)
		categoryMappings := migrateCategories(processedCategories)
		migrateQuestions(categoryMappings)
		// migrateAnswers
	}

	app.Run(os.Args)
}
