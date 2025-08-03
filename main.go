package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gocolly/colly"
	"github.com/gorilla/mux"
)

type PostRequest struct {
	ProductTitle string `json:"productTitle"`
	WowDealPrice string `json:"wowDealPrice"`
	ProductUrl   string `json:"productUrl"`
}
type GetResponse struct {
	FlipkartPrice     string `json:"flipkartPrice"`
	WowDealPrice      string `json:"wowDealPrice"`
	ProductImgUrl     string `json:"productImgUrl"`
	SavingsPercentage int16  `json:"savingsPercentage"`
}

var db = make(map[string]PostRequest) // using this as in-memory database

func main() {
	fmt.Println("Hello, world!")
	r := mux.NewRouter()
	r.HandleFunc("/api/health", checkHealth).Methods("GET")
	r.HandleFunc("/api/prices", getPrices).Methods("POST")
	r.HandleFunc("/api/prices/{productTitle}", getProduct).Methods("GET")

	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func checkHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func getPrices(w http.ResponseWriter, r *http.Request) {
	var req PostRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db[req.ProductTitle] = req

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Product added successfully!"))
}

// <div class="Nx9bqj CxhGGd">₹33,999</div> — Flipkart price element
// <div class="_4WELSP _6lpKCl" style="height: inherit; width: inherit;"><img loading="eager" class="DByuf4 IZexXJ jLEJ7H"></div>// image element
func getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productTitle := vars["productTitle"]
	product, exists := db[productTitle]
	if !exists {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	c := colly.NewCollector(colly.Async(true))
	var res GetResponse
	done := make(chan struct{}) // channel to signale completion of scraping

	// capturing price
	c.OnHTML("div.Nx9bqj", func(e *colly.HTMLElement) {
		fmt.Println(res.FlipkartPrice)
		if res.FlipkartPrice == "" { // using this condition it returns two different prices

			res.FlipkartPrice = e.Text
			fmt.Println("Flipkart Price:", res.FlipkartPrice)
		}
	})
	// capturing image
	c.OnHTML("div._4WELSP._6lpKCl img", func(e *colly.HTMLElement) {
		if res.ProductImgUrl == "" {
			res.ProductImgUrl = e.Attr("src")
			fmt.Println("Image URL:", res.ProductImgUrl)
		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	c.OnScraped(func(r *colly.Response) {
		done <- struct{}{}
	})
	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Error:", err)
		http.Error(w, "Failed to scrape product", http.StatusInternalServerError)

	})

	// err := c.Visit(product.ProductUrl)
	// if err != nil {
	// 	http.Error(w, "Failed to visit product URL", http.StatusInternalServerError)
	// 	return
	// }
	go func() {
		_ = c.Visit(product.ProductUrl)
		c.Wait()
	}()

	select {
	case <-done:

	case <-time.After(time.Second * 15):
		http.Error(w, "Timeout", http.StatusInternalServerError)
		return
	}
	res.WowDealPrice = product.WowDealPrice
	numPrice1, err1 := cleanPrice(res.FlipkartPrice)
	numPrice2, err2 := cleanPrice(product.WowDealPrice)

	if err1 == nil && err2 == nil && numPrice1 > 0 {
		res.SavingsPercentage = int16((numPrice1 - numPrice2) / numPrice1 * 100)
	} else {
		res.SavingsPercentage = 0
	}

	fmt.Println("Final Response:", res)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

// helper function
func cleanPrice(price string) (float64, error) {
	re := regexp.MustCompile(`[^\d.]`)
	clean := re.ReplaceAllString(price, "")
	return strconv.ParseFloat(clean, 64)
}
