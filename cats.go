package main

import (
	"encoding/json"
	"net/http"

	// Генерация уникальных идентификаторов
	"github.com/chilts/sid"
	// Коннектор к Tarantool
	"github.com/tarantool/go-tarantool"
)

// Структура для сериализации гео объектов в/из Tarantool
type CatObject struct {
	Id          string     `json:"id"`
	Coordinates [2]float64 `json:"coordinates"`
	Name        string     `json:"name"`
}

func main() {
	opts := tarantool.Opts{User: "storage", Pass: "passw0rd"}
	conn, err := tarantool.Connect("127.0.0.1:3301", opts)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	opts = tarantool.Opts{User: "storage", Pass: "passw0rd"}
	readconn, err := tarantool.Connect("127.0.0.1:3302", opts)
	if err != nil {
		panic(err)
	}
	defer readconn.Close()

	http.HandleFunc("/cat.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "cat.png")
	})

	// В корневом эндпоинте отдаём пользователю фронтенд
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Эндпоинт для сохранения маркера
	http.HandleFunc("/put", func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		obj := &CatObject{}
		err := dec.Decode(obj)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Генерируем уникальный идентификатор маркера
		if obj.Id == "" {
			obj.Id = sid.IdHex()
		}
		var tuples []CatObject
		// Вставляем новый маркер
		err = conn.ReplaceTyped("cats", []interface{}{obj.Id, obj.Coordinates, obj.Name}, &tuples)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		enc := json.NewEncoder(w)
		enc.Encode(tuples)
		r.Body.Close()
	})

	// Отдаём маркеры для указанного в url региона
	http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		rect, ok := r.URL.Query()["rect"]
		if !ok || len(rect) < 1 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var arr []float64
		err := json.Unmarshal([]byte(rect[0]), &arr)
		if err != nil {
			panic(err)
		}

		// Запрашивает 1000 маркеров, которые находятся в регионе rect
		var tuples []CatObject
		err = readconn.SelectTyped("cats", "spatial", 0, 1000, tarantool.IterLe,
			arr,
			&tuples)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		enc := json.NewEncoder(w)
		enc.Encode(tuples)
		r.Body.Close()
	})

	// Запускаем http сервер на локальном адресе
	err = http.ListenAndServe("127.0.0.1:8080", nil)
	if err != nil {
		panic(err)
	}

}
