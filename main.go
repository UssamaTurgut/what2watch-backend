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

type Movie struct {
	ID      string
	Title   string
	Banners string
	Year    string
	Posters string
	Url     string
	Genres  string
}

type Watchlist struct {
	Movies []Movie
}

type User struct {
	Watchlist Watchlist
}

type Session struct {
	user   User
	friend User
}

type AlgorithmHelper struct {
	db *dbx.DB
}

func (a *AlgorithmHelper) algorithm(user User, friend User) []Movie {
	// var sharedList []Movie

	// //Step 1 find all common movies
	// //go over all Movies in user's watchlist

	// for _, movie := range user.Watchlist.Movies {
	// 	for _, friendMovie := range friend.Watchlist.Movies {
	// 		if movie.ID == friendMovie.ID {
	// 			sharedList = append(sharedList, movie)
	// 		}
	// 	}
	// }

	//Step 2B get seperate favourite Genres from both friends
	userGenreList := getFavouriteGenres(user)
	friendGenreList := getFavouriteGenres(friend)

	//Step 3B find compromise between both Lists
	commonScoreMap := make(map[string]int)

	for i := 0; i < len(userGenreList); i++ {
		commonScoreMap[userGenreList[i]] = i + indexOf(userGenreList[i], friendGenreList)
	}

	keys := make([]string, 0, len(commonScoreMap))
	for key := range commonScoreMap {
		keys = append(keys, key)
	}

	//sort from low to high -> keys sorted by importance of Genre
	sort.SliceStable(keys, func(i, j int) bool {
		return commonScoreMap[keys[i]] < commonScoreMap[keys[j]]
	})

	//Step 4B determine List of Movies according to Genre Importance
	// map[genre]rating, with higher rating being better
	actualScoreMap := make(map[string]int)
	for i := 0; i < len(commonScoreMap); i++ {
		actualScoreMap[keys[i]] = len(commonScoreMap) - commonScoreMap[keys[i]]
	}

	fmt.Printf("---------------------\n")
	fmt.Printf("%#v", actualScoreMap)

	var result []Movie = make([]Movie, 0, len(actualScoreMap))

	actualKeys := make([]string, 0, len(commonScoreMap))

	for actualKey := range commonScoreMap {
		actualKeys = append(actualKeys, actualKey)
	}

	for i := 0; i < len(actualScoreMap); i++ {
		result = append(result, a.getTen(actualKeys[i])...)
	}

	//result holds all the Movies now we need to sort them

	var endResult []Movie

	movieScoreMap := make(map[Movie]int)

	for i := 0; i < len(result); i++ {
		movieScoreMap[result[i]] = getScore(result[i], actualScoreMap)
	}

	movieKeys := make([]Movie, 0, len(result))

	for movieKey := range movieScoreMap {
		movieKeys = append(movieKeys, movieKey)
	}

	sort.SliceStable(movieKeys, func(i, j int) bool {
		return movieScoreMap[movieKeys[i]] < movieScoreMap[movieKeys[j]]
	})

	var limit = len(movieKeys)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		endResult = append(endResult, movieKeys[i])
	}

	return endResult

}

func getScore(movie Movie, actualScore map[string]int) int {

	score := 0

	split := strings.Split(movie.Genres, ",")

	for i := 0; i < len(split); i++ {
		score += actualScore[strings.TrimSpace(split[i])]
	}

	return score
}

func (a *AlgorithmHelper) getTen(genre string) (result []Movie) {
	rows, err := a.db.Select(
		"m.id", "m.title", "m.banners", "m.posters", "m.url", "m.genres", "m.year",
	).From("movies m").Where(dbx.Like(
		"m.genres",
		genre,
	)).OrderBy("RANDOM()").Limit(10).Rows()
	if err != nil {
		log.Println(err)
		return nil
	}

	for rows.Next() {
		var movie Movie
		err = rows.Scan(&movie.ID, &movie.Title, &movie.Banners, &movie.Posters, &movie.Url, &movie.Genres, &movie.Year)
		if err != nil {
			return nil
		}
		result = append(result, movie)
	}

	return result
}

// returns sorted List of fav genres, first element is most favourite
func getFavouriteGenres(user User) []string {
	var genres = make(map[string]int)

	for i := 0; i < len(user.Watchlist.Movies); i++ {
		split := strings.Split(user.Watchlist.Movies[i].Genres, ",")

		for j := 0; j < len(split); j++ {
			genres[strings.TrimSpace(split[j])]++
		}
	}

	keys := []string{}
	for key := range genres {
		keys = append(keys, key)
	}

	sort.SliceStable(keys, func(i, j int) bool {
		return genres[keys[i]] > genres[keys[j]]
	})

	return keys
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
				res, err := app.DB().NewQuery("Update sessions Set partner = {:partner} Where id = {:id}").Bind(dbx.Params{
					"id":      id,
					"partner": authRecord.BaseModel.Id,
				}).Execute()
				if err != nil {
					return err
				}
				count, err := res.RowsAffected()
				if count == 0 || err != nil {
					return apis.NewNotFoundError("Session not found", nil)
				}

				var sessionCreatorID, sessionPartnerID string
				err = app.DB().NewQuery("Select creator, partner From sessions Where id = {:id}").Bind(dbx.Params{
					"id": id,
				}).Row(&sessionCreatorID, &sessionPartnerID)
				if err != nil {
					return err
				}

				// Fetch both users watchlist
				var creatorWatchlist []Movie
				rows, err := app.DB().NewQuery("Select m.id, m.title, m.banners, m.posters, m.url, m.genres, m.year From movies m, watchlist w Where user = {:user} and m.id = w.movie").Bind(dbx.Params{
					"user": sessionCreatorID,
				}).Rows()
				if err != nil {
					return err
				}

				for rows.Next() {
					var movie Movie
					err = rows.Scan(&movie.ID, &movie.Title, &movie.Banners, &movie.Posters, &movie.Url, &movie.Genres, &movie.Year)
					if err != nil {
						return err
					}
					creatorWatchlist = append(creatorWatchlist, movie)
				}

				var partnerWatchlist []Movie
				rows, err = app.DB().NewQuery("Select m.id, m.title, m.banners, m.posters, m.url, m.genres, m.year From movies m, watchlist w Where user = {:user} and m.id = w.movie").Bind(dbx.Params{
					"user": sessionPartnerID,
				}).Rows()
				if err != nil {
					return err
				}

				for rows.Next() {
					var movie Movie
					err = rows.Scan(&movie.ID, &movie.Title, &movie.Banners, &movie.Posters, &movie.Url, &movie.Genres, &movie.Year)
					if err != nil {
						return err
					}
					partnerWatchlist = append(partnerWatchlist, movie)
				}

				var a = &AlgorithmHelper{
					db: app.DB(),
				}

				recommendation := a.algorithm(
					User{
						Watchlist: Watchlist{
							Movies: creatorWatchlist,
						},
					},
					User{
						Watchlist: Watchlist{
							Movies: partnerWatchlist,
						},
					},
				)

				return c.JSON(http.StatusOK, map[string]interface{}{
					"recommendation": recommendation,
				})
			},
			Middlewares: []echo.MiddlewareFunc{
				apis.ActivityLogger(app),
			},
		})

		return nil
	})

	var movies = []Movie{
		{
			ID:      "1",
			Title:   "The Shawshank Redemption",
			Banners: "https://image.tmdb.org/t/p/w1280/9O7gLzmreU0nGkIB6K3BsJbzvNv.jpg",
			Year:    "1994",
			Posters: "https://image.tmdb.org/t/p/w500/q6y0Go1tsGEsmtFryDOJo3dEmqu.jpg",
			Url:     "https://www.themoviedb.org/movie/278",
			Genres:  "Drama",
		},
		{
			ID:      "2",
			Title:   "The Godfather",
			Banners: "https://image.tmdb.org/t/p/w1280/rPdtLWNsZmAtoZl9PK7S2wE3qiS.jpg",
			Year:    "1972",
			Posters: "https://image.tmdb.org/t/p/w500/rPdtLWNsZmAtoZl9PK7S2wE3qiS.jpg",
			Url:     "https://www.themoviedb.org/movie/238",
			Genres:  "Crime, Drama",
		},
		{
			ID:      "3",
			Title:   "The Godfather: Part II",
			Banners: "https://image.tmdb.org/t/p/w1280/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			Year:    "1974",
			Posters: "https://image.tmdb.org/t/p/w500/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			Url:     "https://www.themoviedb.org/movie/240",
			Genres:  "Crime, Drama",
		},
		{
			ID:      "4",
			Title:   "The Dark Knight",
			Banners: "https://image.tmdb.org/t/p/w1280/1hRoyzDtpgMU7Dz4JF22RANzQO7.jpg",
			Year:    "2008",
			Posters: "https://image.tmdb.org/t/p/w500/qJ2tW6WMUDux911r6m7haRef0WH.jpg",
			Url:     "https://www.themoviedb.org/movie/155",
			Genres:  "Drama, Crime",
		},
	}

	var movies2 = []Movie{
		{
			ID:      "5",
			Title:   "12 Angry Dogs",
			Banners: "https://image.tmdb.org/t/p/w1280/3W0v956XxSG5xgm7LB6qu8ExYJ2.jpg",
			Year:    "1957",
			Posters: "https://image.tmdb.org/t/p/w500/3W0v956XxSG5xgm7LB6qu8ExYJ2.jpg",
			Url:     "https://www.themoviedb.org/movie/278",
			Genres:  "Drama",
		},
		{
			ID:      "6",
			Title:   "The Godfather: Part III",
			Banners: "https://image.tmdb.org/t/p/w1280/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			Year:    "1990",
			Posters: "https://image.tmdb.org/t/p/w500/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			Url:     "https://www.themoviedb.org/movie/240",
			Genres:  "Crime, Drama",
		},
		{
			ID:      "7",
			Title:   "The Dark Knight Rises",
			Banners: "https://image.tmdb.org/t/p/w1280/1hRoyzDtpgMU7Dz4JF22RANzQO7.jpg",
			Year:    "2012",
			Posters: "https://image.tmdb.org/t/p/w500/qJ2tW6WMUDux911r6m7haRef0WH.jpg",
			Url:     "https://www.themoviedb.org/movie/155",
			Genres:  "Drama, Crime",
		},
	}

	var watchlist = []Watchlist{
		{
			Movies: movies,
		},
		{
			Movies: movies2,
		},
	}

	var user1 = User{watchlist[0]}
	var user2 = User{watchlist[1]}

	var session = Session{user1, user2}

	helper := &AlgorithmHelper{
		db: app.DB(),
	}
	// helper.algorithm(session.user, session.friend)
	_ = helper
	_ = session

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
