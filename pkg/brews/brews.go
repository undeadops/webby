package brews

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var (
	brewsUrl = "https://api.openbrewerydb.org/breweries"
)

type Brewery struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	BreweryType    string      `json:"brewery_type"`
	Street         string      `json:"street"`
	Address2       interface{} `json:"address_2"`
	Address3       interface{} `json:"address_3"`
	City           string      `json:"city"`
	State          string      `json:"state"`
	CountyProvince interface{} `json:"county_province"`
	PostalCode     string      `json:"postal_code"`
	Country        string      `json:"country"`
	Longitude      string      `json:"longitude"`
	Latitude       string      `json:"latitude"`
	Phone          string      `json:"phone"`
	WebsiteURL     string      `json:"website_url"`
	UpdatedAt      time.Time   `json:"updated_at"`
	CreatedAt      time.Time   `json:"created_at"`
}

func GetBrews(city string) ([]Brewery, error) {
	// Error checking of city should happen, but meh
	url := fmt.Sprintf("%s?by_city=%s&per_page=5", brewsUrl, city)
	fmt.Println(url)
	response, err := http.Get(url)
	if err != nil {
		return []Brewery{}, err
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	responseObject := []Brewery{}
	json.Unmarshal(responseData, &responseObject)

	return responseObject, nil
}
