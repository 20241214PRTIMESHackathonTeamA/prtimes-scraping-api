package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
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

type LikeCountResponse struct {
	Data struct {
		LikeCount int `json:"like_count"`
	} `json:"data"`
}

type ResponseItem struct {
	CorporationName string `json:"corporationName"`
	PublishedDate   string `json:"publishdDatetime"`
	ThumbnailURL    string `json:"thumbnailUrl"`
	PostURL         string `json:"postUrl"`
	Title           string `json:"title"`
	LikeCount       int    `json:"likeCount"`
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

func fetchLikeCount(releaseID string) (int, error) {
	url := fmt.Sprintf("https://prtimes.jp/api/press_release.php/press_release/%s/like_count", releaseID)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var likeResp LikeCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&likeResp); err != nil {
		return 0, err
	}

	return likeResp.Data.LikeCount, nil
}

func extractReleaseID(releaseURL string) string {
	re := regexp.MustCompile(`/main/html/rd/p/([0-9]+)\.([0-9]+)\.html`)
	matches := re.FindStringSubmatch(releaseURL)
	if len(matches) == 3 {
		return fmt.Sprintf("%s.%s", matches[1], matches[2])
	}
	return ""
}

func handlePRTimesPosts(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		http.Error(w, "keyword query parameter is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 0
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			http.Error(w, "limit query parameter must be a positive integer", http.StatusBadRequest)
			return
		}
	}

	// Fetch the first page to determine the total number of pages
	firstPageData, err := fetchPRTimesData(keyword, 1)
	if err != nil {
		http.Error(w, "Failed to fetch data from PR TIMES API", http.StatusInternalServerError)
		log.Println("Error fetching data:", err)
		return
	}

	totalPages := firstPageData.Data.LastPage
	var results []ResponseItem
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

			for _, release := range prTimesData.Data.ReleaseList {
				releaseID := extractReleaseID(release.ReleaseURL)
				likeCount, err := fetchLikeCount(releaseID)
				if err != nil {
					log.Println("Error fetching like count for", releaseID, ":", err)
					likeCount = 0
				}

				item := ResponseItem{
					CorporationName: release.CompanyName,
					PublishedDate:   parseReleaseDate(release.ReleasedAt),
					ThumbnailURL:    release.ThumbnailURL,
					PostURL:         "https://prtimes.jp" + release.ReleaseURL,
					Title:           release.Title,
					LikeCount:       likeCount,
				}

				mu.Lock()
				results = append(results, item)
				mu.Unlock()
			}
		}(page)
	}

	wg.Wait()

	// LikeCountで降順ソート
	sort.Slice(results, func(i, j int) bool {
		return results[i].LikeCount > results[j].LikeCount
	})

	// Limitに応じてデータをカット
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	// Write the JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Println("Error encoding response:", err)
		return
	}
}

func parseReleaseDate(dateStr string) string {
	// 「〇時間前」の形式を処理
	reHours := regexp.MustCompile(`(\d+)時間前`)
	if matches := reHours.FindStringSubmatch(dateStr); len(matches) == 2 {
		hoursAgo, err := strconv.Atoi(matches[1])
		if err == nil {
			parsedTime := time.Now().Add(-time.Duration(hoursAgo) * time.Hour)
			return parsedTime.Format("2006年01月02日 15:04")
		}
	}

	// 「〇分前」の形式を処理
	reMinutes := regexp.MustCompile(`(\d+)分前`)
	if matches := reMinutes.FindStringSubmatch(dateStr); len(matches) == 2 {
		minutesAgo, err := strconv.Atoi(matches[1])
		if err == nil {
			parsedTime := time.Now().Add(-time.Duration(minutesAgo) * time.Minute)
			return parsedTime.Format("2006年01月02日 15:04")
		}
	}

	// 絶対時間の形式を処理 (例: 2024年12月3日 09時00分)
	absoluteFormat := "2006年1月2日 15時04分" // 月や日が1桁の場合も対応
	parsedTime, err := time.Parse(absoluteFormat, dateStr)
	if err == nil {
		return parsedTime.Format("2006年01月02日 15:04")
	}

	// 処理できない場合は現在時刻を返す
	log.Println("Unable to parse date:", dateStr)
	return time.Now().Format("2006年01月02日 15:04")
}

func main() {
	http.HandleFunc("/prtimes_posts", handlePRTimesPosts)
	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
