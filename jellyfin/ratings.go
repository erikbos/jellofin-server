package jellyfin

import "errors"

type Score struct {
	Name     string
	Score    int
	SubScore int
}

// GetAllParentalRatings retrieves all parental rating information.
func GetAllParentalRatings() []Score {
	items := make([]Score, 0, len(parentalRatingsResponse))
	for _, item := range parentalRatingsResponse {
		items = append(items, item)
	}
	return items
}

// LookupParentalRating retrieves the parental rating information for a given name.
func LookupParentalRating(name string) (Score, error) {
	item, ok := parentalRatingsResponse[name]
	if !ok {
		return Score{}, errors.New("parental rating not found")
	}
	return item, nil
}

var parentalRatingsResponse = map[string]Score{
	"Unrated":    {Name: "Unrated"},
	"Approved":   {Name: "Approved", Score: 0},
	"G":          {Name: "G", Score: 0},
	"TV-G":       {Name: "TV-G", Score: 0},
	"TV-Y":       {Name: "TV-Y", Score: 0},
	"TV-Y7":      {Name: "TV-Y7", Score: 7},
	"TV-Y7-FV":   {Name: "TV-Y7-FV", Score: 7, SubScore: 1},
	"PG":         {Name: "PG", Score: 10},
	"TV-PG":      {Name: "TV-PG", Score: 10},
	"TV-PG-D":    {Name: "TV-PG-D", Score: 10, SubScore: 1},
	"TV-PG-L":    {Name: "TV-PG-L", Score: 10, SubScore: 1},
	"TV-PG-S":    {Name: "TV-PG-S", Score: 10, SubScore: 1},
	"TV-PG-V":    {Name: "TV-PG-V", Score: 10, SubScore: 1},
	"TV-PG-DL":   {Name: "TV-PG-DL", Score: 10, SubScore: 1},
	"TV-PG-DS":   {Name: "TV-PG-DS", Score: 10, SubScore: 1},
	"TV-PG-DV":   {Name: "TV-PG-DV", Score: 10, SubScore: 1},
	"TV-PG-LS":   {Name: "TV-PG-LS", Score: 10, SubScore: 1},
	"TV-PG-LV":   {Name: "TV-PG-LV", Score: 10, SubScore: 1},
	"TV-PG-SV":   {Name: "TV-PG-SV", Score: 10, SubScore: 1},
	"TV-PG-DLS":  {Name: "TV-PG-DLS", Score: 10, SubScore: 1},
	"TV-PG-DLV":  {Name: "TV-PG-DLV", Score: 10, SubScore: 1},
	"TV-PG-DSV":  {Name: "TV-PG-DSV", Score: 10, SubScore: 1},
	"TV-PG-LSV":  {Name: "TV-PG-LSV", Score: 10, SubScore: 1},
	"TV-PG-DLSV": {Name: "TV-PG-DLSV", Score: 10, SubScore: 1},
	"PG-13":      {Name: "PG-13", Score: 13},
	"TV-14":      {Name: "TV-14", Score: 14},
	"TV-14-D":    {Name: "TV-14-D", Score: 14, SubScore: 1},
	"TV-14-L":    {Name: "TV-14-L", Score: 14, SubScore: 1},
	"TV-14-S":    {Name: "TV-14-S", Score: 14, SubScore: 1},
	"TV-14-V":    {Name: "TV-14-V", Score: 14, SubScore: 1},
	"TV-14-DL":   {Name: "TV-14-DL", Score: 14, SubScore: 1},
	"TV-14-DS":   {Name: "TV-14-DS", Score: 14, SubScore: 1},
	"TV-14-DV":   {Name: "TV-14-DV", Score: 14, SubScore: 1},
	"TV-14-LS":   {Name: "TV-14-LS", Score: 14, SubScore: 1},
	"TV-14-LV":   {Name: "TV-14-LV", Score: 14, SubScore: 1},
	"TV-14-SV":   {Name: "TV-14-SV", Score: 14, SubScore: 1},
	"TV-14-DLS":  {Name: "TV-14-DLS", Score: 14, SubScore: 1},
	"TV-14-DLV":  {Name: "TV-14-DLV", Score: 14, SubScore: 1},
	"TV-14-DSV":  {Name: "TV-14-DSV", Score: 14, SubScore: 1},
	"TV-14-LSV":  {Name: "TV-14-LSV", Score: 14, SubScore: 1},
	"TV-14-DLSV": {Name: "TV-14-DLSV", Score: 14, SubScore: 1},
	"R":          {Name: "R", Score: 17},
	"NC-17":      {Name: "NC-17", Score: 17, SubScore: 1},
	"TV-MA":      {Name: "TV-MA", Score: 17, SubScore: 1},
	"TV-MA-L":    {Name: "TV-MA-L", Score: 17, SubScore: 1},
	"TV-MA-S":    {Name: "TV-MA-S", Score: 17, SubScore: 1},
	"TV-MA-V":    {Name: "TV-MA-V", Score: 17, SubScore: 1},
	"TV-MA-LS":   {Name: "TV-MA-LS", Score: 17, SubScore: 1},
	"TV-MA-LV":   {Name: "TV-MA-LV", Score: 17, SubScore: 1},
	"TV-MA-SV":   {Name: "TV-MA-SV", Score: 17, SubScore: 1},
	"TV-MA-LSV":  {Name: "TV-MA-LSV", Score: 17, SubScore: 1},
	"TV-X":       {Name: "TV-X", Score: 18},
	"TV-AO":      {Name: "TV-AO", Score: 18},
	"21":         {Name: "21", Score: 21},
	"XXX":        {Name: "XXX", Score: 1000},
	"Banned":     {Name: "Banned", Score: 1001},
}
