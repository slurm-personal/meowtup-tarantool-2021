package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/chilts/sid"
	"github.com/tarantool/go-tarantool"
)

var names = []string{
	"Боня",
	"Ася",
	"Алиса",
	"Багира",
	"Бусинка",
	"Буся",
	"Аврора",
	"Муся",
	"Агния",
	"Ева",
	"Мася",
	"Агата",
	"Василиса",
	"Соня",
	"Агаша",
	"Мурка",
	"Муська",
	"Нюша",
	"Бася",
	"Симка",
	"Абракадабра",
	"Ангел",
	"Багирка",
	"Аба",
	"Анабель",
	"Абби",
	"Сима",
	"Ванесса",
	"Адель",
	"Дымка",
	"Абигель",
	"Бакси",
	"Барселона",
	"Масяня",
	"Абалина",
	"Даша",
	"Гера",
	"Агнесса",
	"Альфа",
	"Бэлла",
	"Амели",
	"Джессика",
	"Айса",
	"Барса",
	"Карамелька",
	"Бан-Ши",
	"Джесси",
	"Ириска",
	"Китти",
	"Агнес",
	"Айрис",
	"Кака",
	"Барсик",
	"Боня",
	"Бакс",
	"Алекс",
	"Бади",
	"Амур",
	"Ебони",
	"Абуссель",
	"Баксик",
	"Жопкинс",
	"Кузя",
	"Персик",
	"Абрек",
	"Абрикос",
	"Тимоша",
	"Авалон",
	"Бабник",
	"Саймон",
	"Бурбузяка",
	"Абу",
	"Марсик",
	"Маркиз",
	"Дымок",
	"Лаки",
	"Симба",
	"Абрамович",
	"Сёма",
	"Пушок",
	"Айс",
	"Бося",
	"Алмаз",
	"Кекс",
	"Басик",
	"Макс",
	"Феликс",
	"Гарфилд",
	"Том",
	"Тиша",
	"Цезарь",
	"Тишка",
	"Мася",
	"Абакан",
	"Лакки",
	"Васька",
	"Адольф",
	"Марсель",
	"Бабасик",
	"Вася",
	"Зевс",
	"Вольт",
	"Адидас",
	"Лео",
}

func randName() string {
	return names[rand.Intn(len(names))]
}

func randFloat(min float64, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

func main() {
	opts := tarantool.Opts{User: "storage", Pass: "passw0rd"}
	conn, err := tarantool.Connect("127.0.0.1:3301", opts)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	conn.Call("box.space.geo:truncate", []interface{}{})

	tr := &http.Transport{
		MaxConnsPerHost: 50,
	}
	defer tr.CloseIdleConnections()
	cl := &http.Client{
		Transport: tr,
	}

	// Питерские координаты
	bounds := []float64{298.02, 148.52, 299.64, 149.20}

	centerx := 299.12
	centery := 148.80

	rand.Seed(time.Now().UnixNano())

	data := make(map[string]map[string]interface{})
	for i := 0; i < 1e3; i++ {
		id := sid.IdHex()
		item := map[string]interface{}{
			"id":          id,
			"coordinates": []float64{randFloat(bounds[0], bounds[2]), randFloat(bounds[1], bounds[3])},
			"name":        randName(),
		}
		data[id] = item

		bytes := new(bytes.Buffer)
		json.NewEncoder(bytes).Encode(item)
		resp, err := cl.Post("http://127.0.0.1:8080/put", "application/json", bytes)
		if err != nil {
			panic(err)
		}
		resp.Body.Close()
	}

	var wg sync.WaitGroup

	parralel := 0
	for step := 0; step < 1e6; step++ {
		for _, source := range data {

			value := make(map[string]interface{})
			value["coordinates"] = source["coordinates"]
			value["id"] = source["id"]
			value["name"] = source["name"]
			coords := value["coordinates"].([]float64)

			parralel = parralel + 1
			wg.Add(1)
			go func() {
				if rand.Int31n(4) == 0 {
					coords[0] = coords[0] + 0.0003
				} else {
					coords[0] = coords[0] - 0.0003
				}
				if rand.Int31n(4) == 0 {
					coords[1] = coords[1] + 0.0003
				} else {
					coords[1] = coords[1] - 0.0003
				}

				if coords[0] > centerx {
					coords[0] = coords[0] - 0.0003
				} else {
					coords[0] = coords[0] + 0.0003
				}
				if coords[1] > centery {
					coords[1] = coords[1] - 0.0003
				} else {
					coords[1] = coords[1] + 0.0003
				}

				if coords[0] < bounds[0] {
					coords[0] = bounds[0]
				}
				if coords[0] > bounds[2] {
					coords[0] = bounds[2]
				}
				if coords[1] < bounds[1] {
					coords[1] = bounds[1]
				}
				if coords[1] > bounds[3] {
					coords[1] = bounds[3]
				}

				value["coordinates"] = coords

				bytes := new(bytes.Buffer)
				json.NewEncoder(bytes).Encode(value)
				resp, err := cl.Post("http://127.0.0.1:8080/put", "application/json", bytes)
				if err != nil {
					panic(err)
				}
				io.Copy(ioutil.Discard, resp.Body)
				resp.Body.Close()
				wg.Done()
			}()
			if parralel == 1 {
				parralel = 0
				wg.Wait()
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}
