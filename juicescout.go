package main

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"os"
	"path"
	"strconv"
	"strings"

	"github.com/antonholmquist/jason"
	"github.com/codegangsta/cli"
	"github.com/joho/godotenv"
	"github.com/parnurzeal/gorequest"
)

var categoriesPath, questionsPath, answersPath, scoutAPI, helpjuiceName, collectionID string

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

// Question is imported from HelpJuice's exported CSV
type Question struct {
	name     string // HelpJuice Question Name
	category int    // HelpJuice Question Category
	id       int    // HelpJuice Question ID
	views    int    // HelpJuice Question Number of views
}

// Answer is imported from HelpJuice's exported CSV
type Answer struct {
	question int    // HelpJuice Answer ID
	body     string // HelpJuice Answer Body
}

// Article stores information required to create an article on HelpScout
type Article struct {
	name       string   // Article name
	text       string   // Content of the article
	categories []string // An array of Category IDs that this article should be associated with
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
	fmt.Println(APIError)
}

// Parse the provided categories CSV
func parseCSV(csvPath string) [][]string {
	filePath := path.Clean(csvPath)
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

func processQuestions(questions [][]string) []Question {
	processedQuestions := []Question{}
	for index, question := range questions {
		// Skip the header column
		if index == 0 {
			continue
		}

		name := question[0]
		category, _ := strconv.Atoi(question[1])
		id, _ := strconv.Atoi(question[2])
		views, _ := strconv.Atoi(question[3])
		processedQuestions = append(processedQuestions, Question{name, category, id, views})

		fmt.Println("Processed questions: ", name)
	}

	return processedQuestions
}

func processAnswers(answers [][]string) []Answer {
	processedAnswers := []Answer{}
	for index, answer := range answers {
		// Skip the header column
		if index == 0 {
			continue
		}

		question, _ := strconv.Atoi(answer[0])
		body := answer[1]
		processedAnswers = append(processedAnswers, Answer{question, body})

		fmt.Println("Processed answers: ", question)
	}

	return processedAnswers
}

func processArticles(categoryMappings []CategoryMapping, questions []Question, answers []Answer) []Article {
	articles := []Article{}
	for _, question := range questions {
		// Find the body
		var questionBody string
		for _, answer := range answers {
			if answer.question == question.id {
				questionBody = answer.body
				break
			}
		}

		var questionCategory []string
		for _, categoryMapping := range categoryMappings {
			if categoryMapping.juiceID == question.category {
				questionCategory = append(questionCategory, categoryMapping.scoutID)
				questionCategory[0] = categoryMapping.scoutID
				break
			}
		}

		articles = append(articles, Article{question.name, questionBody, questionCategory})
	}

	fmt.Println("Processed articles!")
	return articles
}

func createOnScout(object string, resource string, name string) (int, string) {
	// _ = "breakpoint"
	request := gorequest.New().SetBasicAuth(scoutAPI, "X")
	resp, body, errs := request.
		Post("https://docsapi.helpscout.net/v1/" + resource).
		Send(object).
		End()
	errsCheck(errs)

	if resp.StatusCode != 201 {
		checkAPIError(body)
	} else {
		fmt.Println("Success! ", name, " has been created on HelpScout.")
	}

	return resp.StatusCode, body
}

func migrateCategories(categories []Category) ([]CategoryMapping, string) {
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
		createOnScout(categoryObject, "categories", category.name)
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

	return categoryMappings, collectionID
}

func migrateArticles(articles []Article, collectionID string) {
	for _, article := range articles {
		articleName := strings.Replace(article.name, "\n", "</br>", -1)
		articleName = strconv.Quote(articleName)
		articleObject := `{"collectionId": "` + collectionID + `", "name": "` + articleName + `", "categories": [`
		articleCategories := ""
		for index, category := range article.categories {
			articleCategories = articleCategories + `"` + category + `"`
			if index != (len(article.categories) - 1) {
				articleCategories = articleCategories + ", "
			}
		}
		articleText := strings.Replace(article.text, "\n", "<br>", -1)
		articleText = strconv.Quote(articleText)

		articleObject = articleObject + articleCategories + `], "text": ` + articleText + `}`

		statusCode, respBody := createOnScout(articleObject, "articles", article.name)
		if statusCode == 400 && (respBody == `{"code":400,"error":"Invalid Json"}` || respBody == `{"code":400,"error":"Content-Type must be set to 'application/json'"}`) {
			fmt.Println(articleObject)
			log.Fatalln("Danger!")
		}
	}
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
			EnvVar:      "CATEGORIES_PATH",
			Destination: &categoriesPath,
		},
		cli.StringFlag{
			Name:        "questionsPath, q",
			Usage:       "Path of the HelpJuice questions.csv file",
			EnvVar:      "QUESTIONS_PATH",
			Destination: &questionsPath,
		},
		cli.StringFlag{
			Name:        "answersPath, a",
			Usage:       "Path of the HelpJuice answers.csv file",
			EnvVar:      "ANSWERS_PATH",
			Destination: &answersPath,
		},
		cli.StringFlag{
			Name:        "helpjuiceName, j",
			Usage:       "Name of the HelpJuice",
			Destination: &helpjuiceName,
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
		if helpjuiceName == "" {
			log.Fatalln("Missing helpjuiceName flag is required")
		}

		fmt.Println("Beginning migration!")
		start := time.Now()
		categories := parseCSV(categoriesPath)
		processedCategories := processCategories(categories)
		categoryMappings, collectionID := migrateCategories(processedCategories)
		questions := parseCSV(questionsPath)
		processedQuestions := processQuestions(questions)
		answers := parseCSV(answersPath)
		processedAnswers := processAnswers(answers)
		processedArticles := processArticles(categoryMappings, processedQuestions, processedAnswers)
		migrateArticles(processedArticles, collectionID)
		end := time.Now()
		fmt.Println("Done!")
		fmt.Print("Took ")
		fmt.Print(end.Sub(start))
	}

	app.Run(os.Args)
}
