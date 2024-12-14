## PRTIMES Scraping API

### About The Project

### Getting Started

required to go version 1.23.3 or higher

```
go mod tidy
go run main.go
```

### API Reference

#### Get PRTIMES Posts

##### Path

```
GET /prtimes_posts
```

#### Query parameters
- keyword: string (Required)

#### Response

```
[
    {
        "corporationName": "株式会社YYYYYY",
        "publishdDatetime": "2024年12月14日",
        "thumbnailUrl": "https://example.com/xxxx"
        "postUrl": "/main/html/rd/p/xxxxxxx.xxxxxxxxxx",
        "title": "ZZZZZの製品をリリースしました",
        "like_count": "100",
    }
]
```
