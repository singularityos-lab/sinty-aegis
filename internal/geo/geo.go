package geo

import (
	"bufio"
	"math"
	"os"
	"strconv"
	"strings"
)

type Point struct {
	Lat float64
	Lon float64
}

type Circle struct {
	Center Point
	Km     float64
}

func Distance(a, b Point) float64 {
	const r = 6371.0088
	la1, la2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dla := (b.Lat - a.Lat) * math.Pi / 180
	dlo := (b.Lon - a.Lon) * math.Pi / 180
	h := math.Sin(dla/2)*math.Sin(dla/2) + math.Cos(la1)*math.Cos(la2)*math.Sin(dlo/2)*math.Sin(dlo/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(h)))
}

func ParsePolygon(s string) []Point {
	var out []Point
	for _, pair := range strings.Fields(s) {
		lat, lon, ok := strings.Cut(pair, ",")
		if !ok {
			return nil
		}
		la, err1 := strconv.ParseFloat(lat, 64)
		lo, err2 := strconv.ParseFloat(lon, 64)
		if err1 != nil || err2 != nil {
			return nil
		}
		out = append(out, Point{la, lo})
	}
	if len(out) < 3 {
		return nil
	}
	return out
}

func ParseCircle(s string) (Circle, bool) {
	center, radius, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok {
		return Circle{}, false
	}
	lat, lon, ok := strings.Cut(center, ",")
	if !ok {
		return Circle{}, false
	}
	la, err1 := strconv.ParseFloat(lat, 64)
	lo, err2 := strconv.ParseFloat(lon, 64)
	km, err3 := strconv.ParseFloat(strings.TrimSpace(radius), 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return Circle{}, false
	}
	return Circle{Point{la, lo}, km}, true
}

func InPolygon(p Point, poly []Point) bool {
	in := false
	j := len(poly) - 1
	for i := range poly {
		a, b := poly[i], poly[j]
		if (a.Lon > p.Lon) != (b.Lon > p.Lon) && p.Lat < (b.Lat-a.Lat)*(p.Lon-a.Lon)/(b.Lon-a.Lon)+a.Lat {
			in = !in
		}
		j = i
	}
	return in
}

func (c Circle) Contains(p Point) bool {
	return Distance(c.Center, p) <= c.Km
}

type Zone struct {
	Country string
	Point   Point
}

func parseISO6709(s string) (Point, bool) {
	cut := strings.LastIndexAny(s, "+-")
	if cut <= 0 {
		return Point{}, false
	}
	conv := func(v string, deg int) (float64, bool) {
		if len(v) < 1+deg+2 {
			return 0, false
		}
		sign := 1.0
		if v[0] == '-' {
			sign = -1
		}
		d, err1 := strconv.Atoi(v[1 : 1+deg])
		m, err2 := strconv.Atoi(v[1+deg : 1+deg+2])
		sec := 0
		if len(v) >= 1+deg+4 {
			sec, _ = strconv.Atoi(v[1+deg+2 : 1+deg+4])
		}
		if err1 != nil || err2 != nil {
			return 0, false
		}
		return sign * (float64(d) + float64(m)/60 + float64(sec)/3600), true
	}
	lat, ok1 := conv(s[:cut], 2)
	lon, ok2 := conv(s[cut:], 3)
	return Point{lat, lon}, ok1 && ok2
}

func ZoneFor(timezone string, tab string) (Zone, bool) {
	f, err := os.Open(tab)
	if err != nil {
		return Zone{}, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 || cols[2] != timezone {
			continue
		}
		p, ok := parseISO6709(cols[1])
		if !ok {
			return Zone{}, false
		}
		country, _, _ := strings.Cut(cols[0], ",")
		return Zone{strings.ToLower(country), p}, true
	}
	return Zone{}, false
}

func LocalTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return strings.TrimPrefix(tz, ":")
	}
	link, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	_, name, ok := strings.Cut(link, "zoneinfo/")
	if !ok {
		return ""
	}
	return name
}
