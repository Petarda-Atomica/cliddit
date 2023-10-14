package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

var searchMode string
var query string
var page int
var Wall bool

type postPreview struct {
	Title            string    `json:"title"`
	Image            string    `json:"image"`
	Votes            int       `json:"votes"`
	Author           string    `json:"author"`
	PostTime         time.Time `json:"postTime"`
	NumberOfComments int       `json:"nrOfComments"`
	Link             string    `json:"link"`
}

type subreddit []postPreview

type comment struct {
	Content  string    `json:"content"`
	Author   string    `json:"author"`
	Votes    int       `json:"votes"`
	PostTime time.Time `json:"postTime"`
	Chain    []comment `json:"comments"`
}

type post struct {
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	Image            string    `json:"image"`
	Votes            int       `json:"votes"`
	Author           string    `json:"author"`
	PostTime         time.Time `json:"postTime"`
	NumberOfComments int       `json:"nrOfComments"`
	Comments         []comment `json:"comments"`
}

type result struct {
	Errors    []error   `json:"ERRORS"`
	Subreddit subreddit `json:"postPreview"`
	PostInfo  []post    `json:"posts"`
}

var collector colly.Collector

func main() {
	// Declare flags
	searchModePTR := flag.String("mode", "subreddit", "Use: \"subreddit\" or \"post\" or \"hybrid\"")
	queryPTR := flag.String("query", "pics", "Use subreddit name or post url")
	pagePTR := flag.Int("page", 0, "Page number")
	WallPTR := flag.Bool("Wall", false, "Debug mode")

	// Parse Flags
	flag.Parse()
	searchMode = *searchModePTR
	query = *queryPTR
	page = *pagePTR
	Wall = *WallPTR
	//fmt.Printf("mode: %s\nquery: %s\npage: %d\n", searchMode, query, page)

	collector = *colly.NewCollector()
	collector.OnRequest(handleVisit)

	var s subreddit
	var p []post
	var e []error

	switch searchMode {
	case "subreddit":
		s = searchReddit()
	case "post":
		p = append(p, searchPost(query))
	case "hybrid":
		s = searchReddit()
		for i := 0; i < 25; i++ {
			p = append(p, searchPost(s[i].Link))
		}
	default:
		e = append(e, fmt.Errorf("Invalid mode: %s", searchMode))
	}

	fabricateOutput(s, p, e)

}

func handleError(err error) {
	if err == nil {
		return
	}
	fmt.Println(err)
	panic(err)
}

func handleVisit(site *colly.Request) {
	if Wall {
		fmt.Println("Visiting: " + site.URL.String())
	}
}

func newSubredditLink(subreddit string, page int) string {
	return fmt.Sprintf("https://old.reddit.com/r/%s/?count=%d", subreddit, page*25)
}

func fabricateOutput(s subreddit, p []post, e []error) {
	output := result{
		Errors:    e,
		Subreddit: s,
		PostInfo:  p,
	}

	// Marshall
	jsonData, err := json.Marshal(output)
	if err != nil {
		handleError(err)
	}

	// Convert the JSON byte slice to a string
	fmt.Println(string(jsonData))
}

/*func walkCommentArea(i int, e *colly.HTMLElement) {
	// Get all of the permalinks
	var permalinks []string
	e.ForEach("div.comment", func(i int, e *colly.HTMLElement) {
		parentClass := e.DOM.Parent().Parent().AttrOr("class", "")
		if !strings.Contains(parentClass, "child") {
			link := "https://old.reddit.com" + e.Attr("data-permalink")
			permalinks = append(permalinks, link)
		}
	})

	fmt.Printf("Links: %d\n", len(permalinks))

	// Isolate links and run async
	var comments []comment
	comments = make([]comment, len(permalinks))
	var wg sync.WaitGroup
	for i := 0; i < len(permalinks); i++ {
		wg.Add(1)
		go runCommentPermalink(permalinks[i], i, &comments, &wg)
	}
	wg.Wait()

}*/

func runCommentPermalink(link string, position int, destination *[]comment, signal *sync.WaitGroup) {
	defer signal.Done()
	c := colly.NewCollector()

	c.OnRequest(handleVisit)

	c.OnHTML("div.comment", func(g *colly.HTMLElement) {
		this := comment{}
		var err error
		if g.DOM.Parent().Parent().HasClass("commentarea") {

			// Get this comment info
			this.Content = ""
			g.ForEach("div.entry", func(i int, e *colly.HTMLElement) {

				e.ForEach("div.md", func(i int, h *colly.HTMLElement) { //* Content
					h.ForEach("p", func(i int, f *colly.HTMLElement) {
						this.Content = this.Content + f.Text + "\n"
					})
				})

				e.ForEach("a.author", func(i int, e *colly.HTMLElement) { //* Author
					this.Author = e.Text
				})

				e.ForEach("span.score", func(i int, e *colly.HTMLElement) { //* Votes
					this.Votes, err = strconv.Atoi(strings.Split(e.Text, " ")[0])
					handleError(err)
				})

				e.ForEach("time.live-timestamp", func(i int, e *colly.HTMLElement) { //* PostTime
					unparsedTime := e.Attr("datetime")
					layout := "2006-01-02T15:04:05-07:00"
					this.PostTime, err = time.Parse(layout, unparsedTime)
					handleError(err)
				})
			})

			// Get comment permalinks
			var permalinks []string
			g.ForEach("div.child", func(i int, h *colly.HTMLElement) {
				h.ForEach("div.comment", func(i int, f *colly.HTMLElement) {
					if f.DOM.Parent().Parent().Parent().Parent().Parent().HasClass("commentarea") {
						permalinks = append(permalinks, "https://old.reddit.com"+f.Attr("data-permalink"))
					}
				})
			})
			//fmt.Print("\n\nData:\n", permalinks, "\n\n")

			// Process permalinks
			comments := []comment{}
			comments = make([]comment, len(permalinks))
			var wg sync.WaitGroup
			(*destination)[position] = this
			for i := 0; i < len(permalinks); i++ {
				wg.Add(1)
				iCopy := i // Create a local copy of i
				go func(iCopy int) {
					runCommentPermalink(permalinks[iCopy], iCopy, &comments, &wg)
				}(iCopy)
			}
			wg.Wait()
			(*destination)[position].Chain = comments
			//fmt.Println(this.Content)
		}
	})

	c.Visit(link)

}

func searchReddit() (s subreddit) {
	collector.OnHTML("div[data-subreddit]", func(e *colly.HTMLElement) {
		var err error
		this := postPreview{}
		this.Votes, err = strconv.Atoi(e.Attr("data-score"))
		handleError(err)
		this.NumberOfComments, err = strconv.Atoi(e.Attr("data-comments-count"))
		handleError(err)
		this.Link = "https://old.reddit.com" + e.Attr("data-permalink")

		/* //! Summary:
		*  title
		// votes
		*  image
		*  author
		*  postTime
		// numberOfComments
		// link
		*/

		e.ForEach("a.title", func(i int, e *colly.HTMLElement) { //* title
			this.Title = e.Text
		})

		e.ForEach("img[src]", func(i int, e *colly.HTMLElement) { //* image
			this.Image = "https:" + e.Attr("src")
		})

		e.ForEach("a.author", func(i int, e *colly.HTMLElement) { //* author
			this.Author = e.Text
		})

		e.ForEach("time.live-timestamp", func(i int, e *colly.HTMLElement) { //* postTime
			unparsedTime := e.Attr("datetime")
			layout := "2006-01-02T15:04:05-07:00"
			this.PostTime, err = time.Parse(layout, unparsedTime)
			handleError(err)
		})

		/* //! Summary:
		// title
		// votes
		// image
		// author
		// postTime
		// numberOfComments
		// link
		*/

		s = append(s, this)
	})

	err := collector.Visit(newSubredditLink(query, page))
	for err != nil {
		handleError(err)
		err = collector.Visit(newSubredditLink(query, page))
	}

	return s
}

func searchPost(link string) (p post) {
	var this post
	var err error
	collector.OnHTML("div.content", func(e *colly.HTMLElement) {
		if e.DOM.Parent().HasClass("sitetable") {
			return
		}

		e.ForEach("a.title", func(i int, e *colly.HTMLElement) { //* Title
			this.Title = e.Text
		})

		e.ForEach("div.md", func(i int, e *colly.HTMLElement) { //* Content
			e.ForEach("p", func(i int, h *colly.HTMLElement) {
				if !h.DOM.Parent().Parent().Parent().Parent().Parent().Parent().HasClass("self") {
					return
				}

				this.Content = h.Text
			})
		})

		e.ForEach("img.preview", func(i int, e *colly.HTMLElement) { //* Image
			this.Image = e.Attr("src")
		})

		e.ForEach("div.score", func(i int, e *colly.HTMLElement) { //* Votes
			this.Votes, err = strconv.Atoi(e.Attr("title"))
			handleError(err)
		})

		e.ForEach("a.author", func(i int, e *colly.HTMLElement) { //* Author
			this.Author = e.Text
		})

		e.ForEach("time.live-timestamp", func(i int, e *colly.HTMLElement) { //* PostTime
			unparsedTime := e.Attr("datetime")
			layout := "2006-01-02T15:04:05-07:00"
			this.PostTime, err = time.Parse(layout, unparsedTime)
			handleError(err)
		})

		e.ForEach("a.comments", func(i int, e *colly.HTMLElement) { //* NumberOfComments
			this.NumberOfComments, err = strconv.Atoi(strings.Split(e.Text, " ")[0])
			handleError(err)
		})

		var permalinks []string
		e.ForEach("div.commentarea", func(i int, f *colly.HTMLElement) {
			// Get all of the permalinks
			f.ForEach("div.comment", func(i int, h *colly.HTMLElement) {
				if h.DOM.Parent().Parent().HasClass("commentarea") {
					link := "https://old.reddit.com" + h.Attr("data-permalink")
					permalinks = append(permalinks, link)
				}
			})

		})
		//fmt.Printf("Permalinks: %d\n", len(permalinks))

		// Isolate links and run async
		var comments []comment
		comments = make([]comment, len(permalinks))
		var wg sync.WaitGroup
		for i := 0; i < len(permalinks); i++ {
			wg.Add(1)
			go runCommentPermalink(permalinks[i], i, &comments, &wg)
		}
		wg.Wait()
		this.Comments = comments
	})

	err = collector.Visit(link)
	for err != nil {
		handleError(err)
		err = collector.Visit(link)
	}

	return this
}

//! FUCK OFF
//! YOU PIECE OF SHIT
