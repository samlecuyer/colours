package main

import (
	"image"
	"image/color"
	_ "image/png"
	_ "image/jpeg"
	_ "image/gif"

	"fmt"
	"log"
	"math"
	"math/rand"

	"net/http"

	"strconv"
	"strings"

	"os"
)

type Data interface {
    Len() int
    Values(i int) (x, y, z float64)
}

type (
	val struct {
		x, y, z float64
	}
	value struct {
		val
		cluster int
	}
	center struct {
		val
		count int
	}
)

type Kmeans struct {
	values []value
	means []center
}

func NewKmeans(data Data) *Kmeans {
	return &Kmeans {
		values: convert(data),
	}
}

func convert(data Data) []value {
        va := make([]value, data.Len())
        for i := 0; i < data.Len(); i++ {
                x, y, z := data.Values(i)
                va[i] = value{val: val{x: x, y: y, z: z}}
        }

        return va
}

func (km *Kmeans) Seed(k int) {
	km.means = make([]center, k)

	km.means[0].val = km.values[rand.Intn(len(km.values))].val
	d := make([]float64, len(km.values))
	for i := 1; i < k; i++ {
		sum := 0.
		for j, v := range km.values {
			_, min := km.nearest(v.val)
			d[j] = min * min
			sum += d[j]
		}
		target := rand.Float64() * sum
		j := 0
		for sum = d[0]; sum	< target; sum += d[j] {
			j++
		}
		km.means[i].val = km.values[j].val
	}
}

func (km *Kmeans) nearest(v val) (c int, min float64) {
	xd, yd, zd := v.x-km.means[0].x,v.y-km.means[0].y,v.z-km.means[0].z
	min = xd*xd + yd*yd + zd*zd

	for i := 1; i < len(km.means); i++ {
		xd, yd, zd = v.x-km.means[i].x,v.y-km.means[i].y,v.z-km.means[i].z
		d := xd*xd + yd*yd + zd*zd
		if d < min {
			min = d
			c = i
		}
	}
	return c, math.Sqrt(min)
}

func (km *Kmeans) Cluster() {
	for i, v := range km.values {
		n, _ := km.nearest(v.val)
		km.values[i].cluster = n
	}

	for {
		for i := range km.means {
			km.means[i] = center{}
		}
		for _, v := range km.values {
			km.means[v.cluster].x += v.x
			km.means[v.cluster].y += v.y
			km.means[v.cluster].z += v.z
			km.means[v.cluster].count++
		}
		for i := range km.means {
			inv := 1 / float64(km.means[i].count)
			km.means[i].x *= inv
			km.means[i].y *= inv
			km.means[i].z *= inv
		}
		deltas := 0
		for i, v := range km.values {
			if n, _ := km.nearest(v.val); n != v.cluster {
				deltas ++
				km.values[i].cluster = n
			}
		}
		if deltas == 0 {
			break
		}
	}
}

func (km *Kmeans) Clusters() (c [][]int) {
	if km.means == nil {
		return
	}
	c = make([][]int, len(km.means))

	for i := range c {
		c[i] = make([]int, 0, km.means[i].count)
	}
	for i, v := range km.values {
		c[v.cluster] = append(c[v.cluster], i)
	}
	return
}

func (km *Kmeans) Means() []center {
	return km.means
}

type Point struct {
	coords []uint8
	n int
	ct int
}

type Points []*Point

func (p Points) Len() int {
	return len(p)
}

func (p Points) Values(i int) (x, y, z float64) {
	return 	float64(p[i].coords[0]),
			float64(p[i].coords[1]), 
			float64(p[i].coords[2])
}

func GetColors(img image.Image) map[color.Color]int {
	size := img.Bounds().Size()
	colorCount := make(map[color.Color]int)
	for x := 0; x < size.X; x++ {
		for y := 0; y < size.Y; y++ {
			c := img.At(x, y)
			colorCount[c] += 1
		}
	}
	return colorCount
}

func GetPoints(img image.Image) []*Point {
	points := make([]*Point, 0)
	for col, count := range GetColors(img) {
		nrgba, _ := color.NRGBAModel.Convert(col).(color.NRGBA)
		points = append(points, 
				&Point{ 
					[]uint8{nrgba.R,nrgba.G,nrgba.B}, 
					3, count, 
				})
	}
	return points
}

func Colorz(filename string, k int) (image.Image, *Kmeans, Points) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0666)
	if err != nil {
		log.Printf("err: %v", err)
		return nil, nil, nil
	}
	img, _, err := image.Decode(file)
	if err != nil {
		log.Printf("err: %v", err)
		return nil, nil, nil
	}
	points := GetPoints(img)

	km := NewKmeans(Points(points))
	km.Seed(k)
	km.Cluster()

	// for _, c := range km.Means() {
 //        fmt.Printf("#%02x%02x%02x \n", 
 //        	uint32(c.x), uint32(c.y), uint32(c.z))
 //    }
    return img, km, points
}

type mask struct {
	i image.Image
	p Points
	k [][]int
}

func (c *mask) ColorModel() color.Model {
    return color.AlphaModel
}

func (c *mask) Bounds() image.Rectangle {
    return c.i.Bounds()
}

func (m *mask) At(x, y int) color.Color {
	nrgba, _ := color.NRGBAModel.Convert(m.i.At(x, y)).(color.NRGBA)
	for _, i := range m.k[0] {
		f := m.p[i]
		col := color.NRGBA{ f.coords[0],f.coords[1],f.coords[2], 255 }
		if nrgba == col {
			return color.Alpha{255}
		}
	}

	return color.Alpha{0}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		k, _ := strconv.Atoi(r.URL.Query().Get("k"))

		fmt.Fprint(w,`
	<div>
	<style type="text/css">
	.color { display: inline-block; width: 20%; }
	.movie, .year { 
		display: inline-block; 
		vertical-align: top;
		width: calc(33% - 28px); 
		margin-bottom: 20px;  
		margin-right: 40px; 
		padding-top: 10px; 
	}
	.movie:nth-of-type(3n), .year:nth-of-type(3n) { margin-right: 0; }
	.movie[data-winner] { border-top: 1px solid gold; }
				</style>
				`)

		imagesDir, _ := os.Open("photos")
		yearNames, _ := imagesDir.Readdirnames(-1)
		for _, yearName := range yearNames {
			if yearName == ".DS_Store" { continue }
			yearDir, _ := os.Open("photos/"+yearName)
			fmt.Fprintf(w, "<div class=\"year\"><h2>%s</h2>", yearName)
			movieNames, _ := yearDir.Readdirnames(-1)
			for _, movie := range movieNames {
				if movie == ".DS_Store" { continue }
				_, km, _ := Colorz("photos/"+yearName+"/"+movie, k)

				if km == nil  {
					if movie != ".DS_Store" {
						log.Printf("Error processing: %s", movie)
					}
					continue
				}
				fmt.Fprint(w, "<div class=\"movie\"")
				if strings.HasPrefix(movie, "01_winner") {
					fmt.Fprint(w, " data-winner ")
				}
				fmt.Fprint(w, ">")
				for _, c := range km.Means() {
		        	fmt.Fprintf(w,"<div class=\"color\" style=\"background: #%02x%02x%02x\" >&nbsp;</div>", 
		        		uint32(c.x), uint32(c.y), uint32(c.z))
		    	}
		    	fmt.Fprint(w, "</div>")
			}
			fmt.Fprint(w, "</div>")
		} 
		fmt.Fprint(w,"</div>")
	})

    log.Fatal(http.ListenAndServe(":8080", nil))

}