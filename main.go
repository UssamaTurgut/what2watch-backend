package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

type Movies struct {
	id      string
	title   string
	banners string
	year    string
	posters string
	url     string
	genres  string
}

type Watchlist struct {
	id     string
	movies []Movies
}

type User struct {
	id        string
	Watchlist Watchlist
}

type Session struct {
	user   User
	friend User
}

func algorithm(user User, friend User) {
	var sharedList []Movies
	sharedInd := 0

	//Step 1 find all common movies
	//go over all Movies in user's watchlist
	for i := 0; i < len(user.Watchlist.movies); i++ {
		userMovie := user.Watchlist.movies[i].id
		//check for that Movie in friends watchlist
		for j := 0; j < len(friend.Watchlist.movies); j++ {
			friendMovie := friend.Watchlist.movies[j].id
			if userMovie == friendMovie {
				sharedList[sharedInd] = user.Watchlist.movies[i]
			}
		}
	}

	//Step 2A iterate through List and find out List of favourite Genres

	/* 	var genres = make(map[string]int)

	   	for _, movie := range sharedList {
	   		split := strings.Split(movie.genres, ",")

	   		for _, genre := range split {
	   			genres[strings.TrimSpace(genre)]++
	   		}
	   	} */

	//Step 2B get seperate favourite Genres from both friends

	userGenreList := getFavouriteGenres(user)
	friendGenreList := getFavouriteGenres(friend)

	fmt.Printf("%#v", userGenreList)
	fmt.Printf("---------------------\n")
	fmt.Printf("%#v", friendGenreList)

	//Step 3B find compromise between both Lists
	commonScoreMap := make(map[string]int)

	for i := 0; i < len(userGenreList); i++ {
		commonScoreMap[userGenreList[i]] = i + indexOf(userGenreList[i], friendGenreList)
	}

	keys := make([]string, 0, len(commonScoreMap))

	for key := range commonScoreMap {
		keys = append(keys, key)
	}

	//sort from low to high -> Map sorted by importance of Genre

	sort.SliceStable(keys, func(i, j int) bool {
		return commonScoreMap[keys[i]] < commonScoreMap[keys[j]]
	})

	//Step 4B determine List of Movies according to Genre Importance

	actualScoreMap := make(map[string]int)

	for i := 0; i < len(commonScoreMap); i++ {
		actualScoreMap[keys[i]] = len(commonScoreMap) + commonScoreMap[keys[i]]
	}

	fmt.Printf("---------------------\n")
	fmt.Printf("%#v", actualScoreMap)

	var result []Movies = make([]Movies, 0, len(actualScoreMap))

	for i := 0; i < len(actualScoreMap); i++ {

	}

	_ = result

}

// returns sorted List of fav genres
func getFavouriteGenres(user User) []string {
	var favGenres []string

	var genres = make(map[string]int)

	for i := 0; i < len(user.Watchlist.movies); i++ {
		split := strings.Split(user.Watchlist.movies[i].genres, ",")

		for j := 0; j < len(split); j++ {
			genres[strings.TrimSpace(split[j])]++
		}
	}

	keys := make([]string, 0, len(genres))

	for key := range genres {
		keys = append(keys, key)
	}

	sort.SliceStable(keys, func(i, j int) bool {
		return genres[keys[i]] > genres[keys[j]]
	})

	favGenres = keys

	return favGenres

}

func indexOf(element string, data []string) int {
	for k, v := range data {
		if element == v {
			return k
		}
	}

	// if its not in the List the Value should be rlly high
	return 20000000 //not found.
}

func main() {
	app := pocketbase.New()
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		// add new "GET /hello" route to the app router (echo)
		e.Router.AddRoute(echo.Route{
			Method: http.MethodPost,
			Path:   "/session/join",
			Handler: func(c echo.Context) error {
				id := c.QueryParam("id")

				// Currently authenticated user
				authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
				if authRecord == nil {
					return apis.NewForbiddenError("Only auth records can access this endpoint", nil)
				}

				// Insert this as partner in the session
				_, err := app.DB().NewQuery("Update sessions Set partner = {:partner} Where id = {:id}").Bind(dbx.Params{
					"id":      id,
					"partner": authRecord.BaseModel.Id,
				}).Execute()
				if err != nil {
					return err
				}

				return c.JSON(http.StatusOK, map[string]interface{}{})
			},
			Middlewares: []echo.MiddlewareFunc{
				apis.ActivityLogger(app),
			},
		})

		return nil
	})

	var movies = []Movies{
		{
			id:      "1",
			title:   "The Shawshank Redemption",
			banners: "https://image.tmdb.org/t/p/w1280/9O7gLzmreU0nGkIB6K3BsJbzvNv.jpg",
			year:    "1994",
			posters: "https://image.tmdb.org/t/p/w500/q6y0Go1tsGEsmtFryDOJo3dEmqu.jpg",
			url:     "https://www.themoviedb.org/movie/278",
			genres:  "Drama",
		},
		{
			id:      "2",
			title:   "The Godfather",
			banners: "https://image.tmdb.org/t/p/w1280/rPdtLWNsZmAtoZl9PK7S2wE3qiS.jpg",
			year:    "1972",
			posters: "https://image.tmdb.org/t/p/w500/rPdtLWNsZmAtoZl9PK7S2wE3qiS.jpg",
			url:     "https://www.themoviedb.org/movie/238",
			genres:  "Crime, Drama",
		},
		{
			id:      "3",
			title:   "The Godfather: Part II",
			banners: "https://image.tmdb.org/t/p/w1280/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			year:    "1974",
			posters: "https://image.tmdb.org/t/p/w500/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			url:     "https://www.themoviedb.org/movie/240",
			genres:  "Crime, Drama",
		},
		{
			id:      "4",
			title:   "The Dark Knight",
			banners: "https://image.tmdb.org/t/p/w1280/1hRoyzDtpgMU7Dz4JF22RANzQO7.jpg",
			year:    "2008",
			posters: "https://image.tmdb.org/t/p/w500/qJ2tW6WMUDux911r6m7haRef0WH.jpg",
			url:     "https://www.themoviedb.org/movie/155",
			genres:  "Drama, Crime",
		},
	}

	var movies2 = []Movies{
		{
			id:      "5",
			title:   "12 Angry Dogs",
			banners: "https://image.tmdb.org/t/p/w1280/3W0v956XxSG5xgm7LB6qu8ExYJ2.jpg",
			year:    "1957",
			posters: "https://image.tmdb.org/t/p/w500/3W0v956XxSG5xgm7LB6qu8ExYJ2.jpg",
			url:     "https://www.themoviedb.org/movie/278",
			genres:  "Drama",
		},
		{
			id:      "6",
			title:   "The Godfather: Part III",
			banners: "https://image.tmdb.org/t/p/w1280/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			year:    "1990",
			posters: "https://image.tmdb.org/t/p/w500/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			url:     "https://www.themoviedb.org/movie/240",
			genres:  "Crime, Drama",
		},
		{
			id:      "7",
			title:   "The Dark Knight Rises",
			banners: "https://image.tmdb.org/t/p/w1280/1hRoyzDtpgMU7Dz4JF22RANzQO7.jpg",
			year:    "2012",
			posters: "https://image.tmdb.org/t/p/w500/qJ2tW6WMUDux911r6m7haRef0WH.jpg",
			url:     "https://www.themoviedb.org/movie/155",
			genres:  "Drama, Crime",
		},
	}

	var watchlist = []Watchlist{
		{
			id:     "20",
			movies: movies,
		},
		{
			id:     "21",
			movies: movies2,
		},
	}

	var user1 = User{"30", watchlist[0]}
	var user2 = User{"31", watchlist[1]}

	var session = Session{user1, user2}

	algorithm(session.user, session.friend)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
