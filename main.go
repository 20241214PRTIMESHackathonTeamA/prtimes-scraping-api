package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type PRTimesResponse struct {
	Data struct {
		CurrentPage int `json:"current_page"`
		LastPage    int `json:"last_page"`
		ReleaseList []struct {
			CompanyName  string `json:"company_name"`
			Title        string `json:"title"`
			ThumbnailURL string `json:"thumbnail_url"`
			ReleaseURL   string `json:"release_url"`
			ReleasedAt   string `json:"released_at"`
		} `json:"release_list"`
	} `json:"data"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type ResponseItem struct {
	CorporationName string `json:"corporationName"`
	PublishedDate   string `json:"publishdDatetime"`
	ThumbnailURL    string `json:"thumbnailUrl"`
	PostURL         string `json:"postUrl"`
	Title           string `json:"title"`
}

func fetchPRTimesData(keyword string, page int) (*PRTimesResponse, error) {
	escapedKeyword := url.QueryEscape(keyword)
	url := fmt.Sprintf("https://prtimes.jp/api/keyword_search.php/search?keyword=%s&page=%d&limit=40", escapedKeyword, page)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var prTimesResp PRTimesResponse
	if err := json.NewDecoder(resp.Body).Decode(&prTimesResp); err != nil {
		return nil, err
	}

	return &prTimesResp, nil
}

func handlePRTimesPosts(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		http.Error(w, "keyword query parameter is required", http.StatusBadRequest)
		return
	}

	// Fetch the first page to determine the total number of pages
	firstPageData, err := fetchPRTimesData(keyword, 1)
	if err != nil {
		http.Error(w, "Failed to fetch data from PR TIMES API", http.StatusInternalServerError)
		log.Println("Error fetching data:", err)
		return
	}

	totalPages := firstPageData.Data.LastPage
	results := []ResponseItem{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Fetch all pages concurrently
	for page := 1; page <= totalPages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			prTimesData, err := fetchPRTimesData(keyword, page)
			if err != nil {
				log.Println("Error fetching page", page, ":", err)
				return
			}

			mu.Lock()
			for _, release := range prTimesData.Data.ReleaseList {
				results = append(results, ResponseItem{
					CorporationName: release.CompanyName,
					PublishedDate:   parseReleaseDate(release.ReleasedAt),
					ThumbnailURL:    release.ThumbnailURL,
					PostURL:         "https://prtimes.jp" + release.ReleaseURL,
					Title:           release.Title,
				})
			}
			mu.Unlock()
		}(page)
	}

	wg.Wait()

	// Write the JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Println("Error encoding response:", err)
		return
	}
}

func parseReleaseDate(dateStr string) string {
	// Here you can implement a proper date parsing and reformatting logic
	// Placeholder: returning the current date for simplicity
	return time.Now().Format("2006年01月02日")
}

func main() {
	http.HandleFunc("/prtimes_posts", handlePRTimesPosts)
	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
