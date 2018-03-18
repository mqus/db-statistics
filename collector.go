package main

import (
	"time"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"fmt"
	"strings"
	"strconv"
	"math/rand"
)

type Traveller struct {
	Bc    uint8  `json:"bc"`
	Typ   string `json:"typ"`
	Alter uint8  `json:"alter"`
}

type DBQuery struct {
	S          int         `json:"s"`
	D          int         `json:"d"`
	Dt         string      `json:"dt"`
	T          string      `json:"t"`
	C          uint8       `json:"c"`
	WithoutICE bool        `json:"ohneICE"`
	Tct        int         `json:"tct"`
	Dur        int         `json:"dur"`
	Travellers []Traveller `json:"travellers"`
	Sv         bool        `json:"sv"`
	V          string      `json:"v"`
	Dir        string      `json:"dir"`
	Bic        bool        `json:"bic"`
	Device     string      `json:"device"`
	Os         string      `json:"os"`
}
type Offer struct {
	T    string   `json:"t"`
	C    string   `json:"c"`
	P    string   `json:"p"`
	Tt   string   `json:"tt"`
	Zb   string   `json:"zb"`
	Arq  string   `json:"arq"`
	Ff   string   `json:"ff"`
	Aix  string   `json:"aix"`
	Sids []string `json:"sids"`
	//	RisIDs []string `json:"risids"` //?
	Pky   string `json:"pky"`
	Angnm string `json:"angnm"`
	KoTxt string `json:"kotxt"`
	//	ueid string `json:"ueid"`
	//	uepr1 string `json:"uepr1"`
	//	uepr2 string `json:"uepr2"`
	//	uepr3 string `json:"uepr3"`
	//	uentg string `json:"uentg"`
}
type DBTime struct {
	Day  string `json:"d"`
	Time string `json:"t"`
	UTC  string `json:"m"`
}

type Train struct {
	Tid     string `json:"tid"`
	Lt      string `json:"lt"`
	LtShort string `json:"ltShort"`
	S       string `json:"s"`
	Sn      string `json:"sn"`
	D       string `json:"d"`
	Dn      string `json:"dn"`
	Tn      string `json:"tn"`
	Eg      string `json:"eg"`
	Dep     DBTime `json:"dep"`
	Arr     DBTime `json:"arr"`
	Pd      string `json:"pd"`
	Pa      string `json:"pa"`
	Rp      bool   `json:"rp"`
	Re      bool   `json:"re"`
	Sp      bool   `json:"sp"`
}

type TrainConnection struct {
	Dir    string  `json:"dir"`
	SID    string  `json:"sid"`
	Dt     string  `json:"dt"`
	Dur    string  `json:"dur"`
	Nt     string  `json:"nt"`
	NRConn bool    `json:"NZVerb"`
	Eg     string  `json:"eg"`
	Trains []Train `json:"trains"`
}

type PeText struct {
	Name string `json:"name"`
	Text string `json:"hinweis"`
}
type Station struct {
	Number string `json:"nummer"`
	Name   string `json:"name"`
}

type DBResponse struct {
	Dir         string                     `json:"dir"`
	Offers      map[string]Offer           `json:"angebote"`
	Connections map[string]TrainConnection `json:"verbindungen"`
	PeTexts     map[string]PeText          `json:"peTexte"`
	Sbf         []Station                  `json:"sbf"`
	Dbf         []Station                  `json:"dbf"`
	Durs        map[string]string          `json:"durs"`
	Prices      map[string]string          `json:"prices"`
	Sp          bool                       `json:"sp"`
	Device      string                     `json:"device"`
}

var stations = map[string]int{
	"Frankfurt(Main)Hbf": 8000105,
	"Berlin Hbf (tief)":  8098160,
	"Hamburg Hbf":        8098549,
	"Muenchen Hbf":       8000261,
	"Dresden Hbf":        8010085,
	"Erfurt Hbf":         8010101,
}

func newQuery(from, to int, t time.Time) DBQuery {
	tlers := [1]Traveller{{
		Bc:    0,   //no bahncard
		Typ:   "E", //adult
		Alter: 25,  //age=25
	}}
	return DBQuery{
		S:          from,                 //start station id
		D:          to,                   //dest station id
		Dt:         t.Format("02.01.06"), //date in DD.MM.YY
		T:          t.Format("15:04"),    //time in HH:mm
		C:          2,                    //class
		WithoutICE: false,                //without ICE (express train)
		Tct:        5,                    //transfer time
		Dur:        1440,                 //search within t+dur in minutes
		Travellers: tlers[:],
		Sv:         true,        //prefer fast routes
		V:          "16040000",  //fixed
		Dir:        "1",         //fixed
		Bic:        false,       //fixed
		Device:     "HANDY",     //fixed
		Os:         "iOS_9.3.1", //fixed
	}
}

func getConnections(from, to string, day time.Time) *DBResponse {
	day = day.Add(-12 * time.Hour).Round(24 * time.Hour) //round to full day
	q := newQuery(stations[from], stations[to], day)
	endpoint := "http://ps.bahn.de/preissuche/preissuche/psc_service.go" //https is slow

	b, err := json.Marshal(q)
	if err != nil {
		log.Fatal("geht nich,JSONify:", err)
	}
	values := url.Values{}
	values.Add("lang", "en")
	values.Add("service", "pscangebotsuche")
	values.Add("data", string(b))

	rq, e := http.NewRequest(http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if e != nil {
		log.Fatal("geht nich,Request:", e)
	}
	rq.Header.Add("Accept", "application/json")
	resp, err0 := http.DefaultClient.Do(rq)
	if err0 != nil {
		log.Fatal("geht nich,RequestDo:", err0)
	}
	decoder := json.NewDecoder(resp.Body)
	r := DBResponse{}
	err1 := decoder.Decode(&r)
	if err1 != nil {
		log.Fatal("geht nich,RespDecode:", err1)
	}

	return &r
}

type SlimConnection struct {
	now       int64
	s         string
	ss        string
	d         string
	ds        string
	dep       int64
	arr       int64
	price     float32
	transfers uint8
}

func NewSlimConnection(offer *Offer, conn TrainConnection) *SlimConnection {
	var first, last Train

	for i, t := range conn.Trains {
		if i != 0 {
			if strings.Compare(t.Dep.UTC, first.Dep.UTC) < 0 {
				first = t
			}
			if strings.Compare(t.Arr.UTC, last.Arr.UTC) > 0 {
				last = t
			}
		} else {
			first = t
			last = t
		}
	}
	dep, _ := strconv.ParseInt(first.Dep.UTC, 10, 64)
	arr, _ := strconv.ParseInt(first.Arr.UTC, 10, 64)
	price, _ := strconv.ParseFloat(strings.Replace(offer.P, ",", ".", -1), 32)
	return &SlimConnection{
		now:       time.Now().UTC().Unix(),
		s:         first.S,
		ss:        first.Sn,
		d:         last.D,
		ds:        last.Dn,
		dep:       dep / 1000,
		arr:       arr / 1000,
		price:     float32(price),
		transfers: uint8(len(conn.Trains) - 1),
	}
}
func main() {
	d := [3][2]string{
		{"Frankfurt(Main)Hbf", "Dresden Hbf"},
		{"Berlin Hbf (tief)", "Muenchen Hbf"},
		{"Hamburg Hbf", "Erfurt Hbf"},
	}
	t, e := time.Parse("02.01.2006 15:04", "16.07.2018 01:00")
	if e != nil {
		log.Fatal("timeerr:", e)
	}
	for _, city := range d {
		//wait for 1-4 minutes
		time.Sleep(time.Minute + time.Duration(rand.Intn(int(time.Minute)*3)))
		for i := time.Duration(0); i < 7; i++ {
			time.Sleep(3 * time.Second)

			r := getConnections(city[0], city[1], t.Add(time.Hour*24*i))
			for _, o := range r.Offers {
				for _, sid := range o.Sids {
					sc := NewSlimConnection(&o, r.Connections[sid])
					fmt.Printf("%d,%d,%d,%d,%d,%.2f,%s,%s,%s,%s,%d",
						stations[city[0]],
						stations[city[1]],
						sc.dep,
						sc.arr,
						sc.now,
						sc.price,
						sc.s,
						sc.d,
						sc.ss,
						sc.ds,
						sc.transfers)
				}
			}
		}
	}

}
