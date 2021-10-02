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
