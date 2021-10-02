# meowtup-tarantool-2021

## Небольшое приложение на Tarantoo, а также golang и html/js.

## Мотивация

Представим, что хочется, чтобы котики чувствовали себя свободно, но и их хозяева не переживали.
Сделаем сервис с отображением котиков на карте.

## Технически

Данные будем хранить и индексировать в Tarantool
Обрабатывать запросы будет в golang
Рисовать карту на html/js

## Начинаем строить приложение

Вот что нам нужно сделать:
	
1. Фронтенд на HTML/JS с Leaflet и OpenStreetMap
    - Виджет с картой
    - События пользователя
    - Запросы к Golang-бекенду
2. Создание Golang приложения
    - Подключение к базе
    - Индексирование
    - Запросы к базе
    - HTTP-сервер
    - HTTP API
3. Конфигурация базы данных

## Фронтенд

Начнем с `html`

Сделаем файл `index.html`

```
touch index.html
```

```html
<html>
</html>
```

Подключим фреймворк для рисования карты — leaflet
```html
<head>
    <title>The Map</title>
    <link rel="stylesheet" href="https://unpkg.com/leaflet@1.7.1/dist/leaflet.css" crossorigin="" />
    <script src="https://unpkg.com/leaflet@1.7.1/dist/leaflet.js" crossorigin=""></script>
    <script src="https://unpkg.com/leaflet-providers@1.0.13/leaflet-providers.js" crossorigin=""></script>
</head>
```

Нарисуем виджет с картой

```html
<body>
    <!-- div для карты -->
    <div id="mapid" style="height:100%"></div>
    <script>
        // Карта
        var mymap = L.map('mapid',
            { 'tap': false })
            .setView([59.95, 30.31], 13)

        // Слой карты с домами, улицами и т.п.
        var osm = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            maxZoom: 19,
            attribution: '&copy <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
        }).addTo(mymap)
    </script>
</body>
```

Добавим слой и добавим на него тестовый маркер просто чтобы убедится

```js   
group = L.layerGroup().addTo(mymap)
L.marker([59.95, 30.31]).addTo(group).bindPopup("Hello World")
```

Должно всё получиться

Создадим функцию, которая будет добавлять объекты на карту по клику мыши

```js
// Обрабатываем нажатие на карту
function onMapClick(e) {
    L.marker(e.latlng).addTo(group).bindPopup("Hello World")
}
mymap.on('click', onMapClick)
```

## Конфигурация базы данных Lua

- Redis говорит: «Сконфигурируй меня с помощью простых команд в файле»
- Mongo говорит: «Сконфигурируй меня с помощью YAML файла»
- Tarantool говорит: «Используй Lua скрипт»


Чтобы запустить Tarantool и подключиться к нему, нам понадобится всего три Lua-функции:
 - `box.cfg`
 - `box.schema.user.create`
 - `box.schema.user.grant`

**box.cfg**

Функция настраивает весь Tarantool. Часть параметров можно задать только один раз при старте. Другую часть можно менять в любой момент времени.

**box.schema.user.create**

Функция создаёт пользователя для удаленной работы.

**box.schema.user.grant**

Функция перечисляет, что можно и что нельзя будет делать пользователю.

Конфигурация Tarantool-а — единственное место, где будет Lua.

Создадим файл init.lua
```
touch init.lua
```

`init.lua`
```lua
-- Открываем порт для доступа по iproto
box.cfg({listen="127.0.0.1:3301"})
-- Создаём пользователя для подключения
box.schema.user.create('storage', {password='passw0rd', if_not_exists=true})
-- Даём все-все права
box.schema.user.grant('storage', 'super', nil, nil, {if_not_exists=true})
```

На одном узле Tarantool находится только одна база данных. Данные складываются в спейсы == таблицы в мире SQL. К данным обязательно строится первичный индекс, а количество вторичных произвольно.

Для хранения маркеров сделаем таблицу:

|id| coordinates| name |
|--|------------|---------|
|string|[double, double]| string|

В поле `id` хранится уникальный идентификатор, который мы сами сгенерируем.
В поле `coordinates` — координаты маркера (массив из двух double).
В поле `name` — строка с кличкой.

```lua
-- создаём таблицу для хранения отзывов на карте
box.schema.space.create('cats', {if_not_exists=true})
box.space.cats:format({
        {name="id", type="string"},
        {name="coordinates", type="array"},
        {name="name", type="string"}
})
-- создаём первичный индекс
box.space.cats:create_index('primary', {
                                parts={{ field="id", type="string" }},
                                type = 'TREE',
                                if_not_exists=true,})
-- создаём индекс для координат
box.space.cats:create_index('spatial', {
                                parts = {{ field="coordinates", type='array'} },
                                type = 'RTREE',
                                unique = false,
                                if_not_exists=true,})
```

Запущу локальную консоль внутри базы данных, для всяких тестовых команд

```lua
require('console').start() os.exit()
```

На этом Lua закачивается.

### Запуск Tarantool 

```
tarantool init.lua
```

## Golang приложение

Создадим файл cats.go

```
touch cats.go
```

```go
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
	Name     string     `json:"name"`
}

func main() {
	opts := tarantool.Opts{User: "storage", Pass: "passw0rd"}
	conn, err := tarantool.Connect("127.0.0.1:3301", opts)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
}
```

Запуск 

```
go run ./cats.go
```

```
go mod init cats
```

```
go get github.com/tarantool/go-tarantool
```

```
go run ./cats.go
```

Добавим хостинг `index.html` файла

```go
// В корневом эндпоинте отдаём пользователю фронтенд
http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "index.html")
})

// Запускаем http сервер на локальном адресе
err = http.ListenAndServe("127.0.0.1:8080", nil)
if err != nil {
    panic(err)
}
```

### Ендпоинт для сохранения кота

```go
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
```

```
go get github.com/chilts/sid
```

```
go run ./cats.go
```

## Отправка котиков из html

```js
function onMapClick(e) {
    var name = window.prompt('Cat name?')
    if (name != null) {
        L.marker(e.latlng).addTo(group).bindPopup(name)

        /*
        * Карта использует систему координат на шаре
        * Tarantool хранит координаты на плоскости
        * Конвертируем из одной системы в другую
        */
        var p = mymap.project(e.latlng, 1)
        
        var cat = {
            "coordinates": [p.x, p.y],
            "name": name,
        }

        fetch("/put", {
            method: "POST",
            body: JSON.stringify(cat)
        })
    }
}
```

## Получение котиков из базы

### Фронтенд

В html добавим функцию для загрузки котиков в рамках экрана

```js
function addCat(cat) {
    var l = mymap.unproject(L.point(cat['coordinates']), 1)

    var name = cat['name']
    // Создаем маркер
    L.marker(l).addTo(group).bindPopup(name)
}
// Обрабатываем json пришедший с сервера
function parse(array) {
    array.forEach(addCat)
}
function errorResponse(error) {
    alert('Error: ' + error)
}
function handleListResponse(res) {
    res.json().then(parse).catch(errorResponse)
}
function onMapMove(e) {
    var bounds = mymap.getBounds()
    var northeast = bounds.getNorthEast()
    var southwest = bounds.getSouthWest()
    var ne = mymap.project(northeast, 1)
    var sw = mymap.project(southwest, 1)
    var options = {
        "rect": JSON.stringify([ne.x, ne.y, sw.x, sw.y]),
    }

    // Отправляем запрос на сервер с получением маркеров
    fetch("/list?" + new URLSearchParams(options))
        .then(handleListResponse)
        .catch(errorResponse)
}
mymap.on('move', onMapMove)
onMapMove()
```

### Бекенд

```go
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
    err = conn.SelectTyped("cats", "spatial", 0, 1000, tarantool.IterLe,
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
```


## Котики должны быть котиками

Добавим файл с котиком

- index.html

```js
var icon = L.icon({
	iconUrl: "data:image/png;base64,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",
	iconSize: [32, 32],
	iconAnchor: [0, 0],
	popupAnchor: [0, 0],
})
```

```js
L.marker(l, {"icon": icon}).addTo(group).bindPopup(name)
```

## Промежуточный итог

Маркеры на карте создаются.
При навигации маркеры загружаются с бека.

### Проблема

При навигации маркеры постоянно добавиляются, даже если уже были.

**Решение**

Выполнять проверку на фронтенде, если маркер уже был отрисован.

```js
var alreadyloaded = {}
function addCat(cat) {
    if (!(cat.id in alreadyloaded)) {
        var l = mymap.unproject(L.point(cat['coordinates']), 1)

        var name = cat['name']
        // Создаем маркер
        L.marker(l, {"icon": icon}).addTo(group).bindPopup(name)
        alreadyloaded[cat.id] = cat
    }
}
```

## А что если коты всегда гуляют сами по себе

Сделаем приложение бота, которое будет рассказывать нам о передвижениях котов. В реальном мире 
это мог бы быть аггрегатор gps координат с ошейников наших питомцев. 

Файл catpaths.go

```
touch catpaths.go
```

- `catpaths.go`

```go
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
```

Запуск

```
go run ./cat_paths.go
```

Теперь если обновлять страницу, то маркеры будут двигаться.

### Автоматическое обновление страницы

Добавим автоматические обновление страницы.

```js
redraw = setInterval(onMapMove, 1)
```

Сделаем изменение координат маркеров.

```js
var alreadyloaded = {}
var popups = {}
function addCat(cat) {
    if (!(cat.id in alreadyloaded)) {
        var l = mymap.unproject(L.point(cat['coordinates']), 1)

        var name = cat['name']
        // Создаем маркер
        popups[cat.id] = L.marker(l, {"icon": icon}).addTo(group).bindPopup(name)
        alreadyloaded[cat.id] = cat
    } else {
        var l = mymap.unproject(L.point(cat['coordinates']), 1)
        popups[cat.id].setLatLng(l)
    }
}
```

## Тестирование нагрузочное

Сделаем файл `cats_test.go` c тестами и проверим 

```
touch cats_test.go
```

```go
package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/http"
	"testing"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func BenchmarkWrite(b *testing.B) {
	tr := &http.Transport{}
	defer tr.CloseIdleConnections()
	cl := &http.Client{
		Transport: tr,
	}

	var x = rand.Float64()
	var y = rand.Float64()
	var data = map[string]interface{}{
		"coordinates": []float64{x, y},
		"name":        RandStringBytes(100),
	}

	for i := 0; i < b.N; i++ {
		data["coordinates"] = []float64{float64(rand.Int31n(1000)) + rand.Float64(), float64(rand.Int31n(1000)) + rand.Float64()}
		bytes := new(bytes.Buffer)
		json.NewEncoder(bytes).Encode(data)
		res, err := cl.Post("http://127.0.0.1:8080/put", "application/json", bytes)
		if err != nil {
			b.Fatal("Post:", err)
		}
		_, err = ioutil.ReadAll(res.Body)
		if err != nil {
			b.Fatal("ReadAll:", err)
		}
	}
}

func BenchmarkRead(b *testing.B) {
	tr := &http.Transport{}
	defer tr.CloseIdleConnections()
	cl := &http.Client{
		Transport: tr,
	}

	for i := 0; i < b.N; i++ {
		bytes, err := json.Marshal([]float64{float64(rand.Int31n(1000)) + rand.Float64(),
            float64(rand.Int31n(1000)) + rand.Float64(),
			float64(rand.Int31n(360)) + rand.Float64(),
            float64(rand.Int31n(360)) + rand.Float64()})
		if err != nil {
			b.Fatal(err)
		}

		res, err := cl.Get("http://127.0.0.1:8080/list?rect=" + string(bytes))
		if err != nil {
			b.Fatal(err)
		}

		var objects []CatObject
		dec := json.NewDecoder(res.Body)
		err = dec.Decode(&objects)
		if err != nil {
			b.Fatal(err)
		}
		res.Body.Close()
	}
}
```

Запуск тестов

```
go test -benchmem -benchtime 10s -bench BenchmarkWrite
```

## Масштабирование

Подключим реплику на чтение
В golang писать будем в мастер, читать будем с реплики

replica.lua

```
touch replica.lua
```

```
box.cfg{work_dir="replica", listen=3302, replication="storage:passw0rd@127.0.0.1:3301"}
```

Создадим рабочую директорию для реплики
```
mkdir replica
```

Запустим реплику
```
tarantool replica.lua
```


Добавим в файл cats.go подключение к readonly реплике

```go
opts = tarantool.Opts{User: "storage", Pass: "passw0rd"}
readconn, err := tarantool.Connect("127.0.0.1:3302", opts)
if err != nil {
	panic(err)
}
defer readconn.Close()
```

```go
err = readconn.SelectTyped("cats", "spatial", 0, 1000, tarantool.IterLe,
			arr,
			&tuples)
```

Запустим котиков.
Потому запустим нагрузочные тесты.

## В заключение


Пользуйтесь Тарантулом, он данные хранит и вам быстро отдаёт.
Tarantool это масштабируемый OLTP.
Clickhouse это масштабируемый OLAP.

Можно писать микросервисы на Tarantool рядом с данными.

Что осталось за кадром:
- Шардирование
- Автоматический фейловер
- Синхронная репликация

Вопросы?
