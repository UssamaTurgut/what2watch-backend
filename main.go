package main

import (
	"database/sql"
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

type AlgorithmHelper struct {
	db *dbx.DB
}

func (a *AlgorithmHelper) algorithm(user User, friend User) []Movie {
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

				var sessionCreatorID string
				var sessionPartnerID sql.NullString
				err := app.DB().NewQuery("Select creator, partner From sessions Where id = {:id}").Bind(dbx.Params{
					"id": id,
				}).Row(&sessionCreatorID, &sessionPartnerID)
				if err != nil {
					return err
				}
				if sessionCreatorID == authRecord.BaseModel.Id {
					return c.JSON(http.StatusOK, map[string]interface{}{
						"role": "creator",
					})
				}

				// If session already has recommendations, we stop here
				var i int
				err = app.DB().Select("count(session)").From("recommendations").Where(dbx.HashExp{
					"session": id,
				}).Row(&i)
				if err != nil {
					return err
				}
				if i > 0 {
					return c.JSON(http.StatusOK, map[string]interface{}{
						"role": "partner",
					})
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

				for _, movie := range recommendation {
					_, err = app.DB().NewQuery("Insert Into recommendations (session, movie) Values ({:session}, {:movie})").Bind(dbx.Params{
						"session": id,
						"movie":   movie.ID,
					}).Execute()
					if err != nil {
						return err
					}
				}

				return c.JSON(http.StatusOK, map[string]interface{}{
					"role":            "partner",
					"recommendations": recommendation,
				})
			},
			Middlewares: []echo.MiddlewareFunc{
				apis.ActivityLogger(app),
			},
		})

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
