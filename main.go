package main

import (
    "bufio"
    "fmt"
    "math"
    "os"
    "sort"
    "strings"
    "sync"
    "time"

    "github.com/go-ping/ping"
)

type Server struct {
    Name    string
    IP      string
    Country string
    City    string
    Lat     float64
    Lon     float64
    AvgRTT  time.Duration
}

type Result struct {
    Server   Server
    Delta    time.Duration
    Distance float64 
}

type Location struct {
    Lat float64
    Lon float64
}

const (
    speedOfLight = 299792.458 
    fiberSpeed   = speedOfLight * 0.67 
    earthRadius  = 6371.0 
)

func AvgPing(ip string, count int) (time.Duration, error) {
    pinger, err := ping.NewPinger(ip)
    if err != nil {
        return 0, err
    }

    pinger.SetPrivileged(true)
    pinger.Count = count
    pinger.Timeout = 10 * time.Second

    err = pinger.Run()
    if err != nil {
        return 0, err
    }

    stats := pinger.Statistics()
    if stats.PacketsRecv == 0 {
        return 0, fmt.Errorf("aucune réponse")
    }

    return stats.AvgRtt, nil
}

func distance(lat1, lon1, lat2, lon2 float64) float64 {
    dLat := (lat2 - lat1) * math.Pi / 180
    dLon := (lon2 - lon1) * math.Pi / 180

    a := math.Sin(dLat/2)*math.Sin(dLat/2) +
        math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
            math.Sin(dLon/2)*math.Sin(dLon/2)

    c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
    return earthRadius * c
}

func rttToDistance(rtt time.Duration) float64 {
    seconds := rtt.Seconds()
    // Division par 2 car RTT = aller-retour
    return (seconds * fiberSpeed) / 2
}

func geoToCartesian(lat, lon float64) (x, y, z float64) {
    latRad := lat * math.Pi / 180
    lonRad := lon * math.Pi / 180

    x = earthRadius * math.Cos(latRad) * math.Cos(lonRad)
    y = earthRadius * math.Cos(latRad) * math.Sin(lonRad)
    z = earthRadius * math.Sin(latRad)
    return
}

func cartesianToGeo(x, y, z float64) (lat, lon float64) {
    lon = math.Atan2(y, x) * 180 / math.Pi
    hyp := math.Sqrt(x*x + y*y)
    lat = math.Atan2(z, hyp) * 180 / math.Pi
    return
}

func trilaterate(s1, s2, s3 Server, d1, d2, d3 float64) Location {
    x1, y1, z1 := geoToCartesian(s1.Lat, s1.Lon)
    x2, y2, z2 := geoToCartesian(s2.Lat, s2.Lon)
    x3, y3, z3 := geoToCartesian(s3.Lat, s3.Lon)

    w1 := 1.0 / (d1 + 1.0) // +1 pour éviter division par 0
    w2 := 1.0 / (d2 + 1.0)
    w3 := 1.0 / (d3 + 1.0)

    totalWeight := w1 + w2 + w3

    xEst := (x1*w1 + x2*w2 + x3*w3) / totalWeight
    yEst := (y1*w1 + y2*w2 + y3*w3) / totalWeight
    zEst := (z1*w1 + z2*w2 + z3*w3) / totalWeight

    norm := math.Sqrt(xEst*xEst + yEst*yEst + zEst*zEst)
    xEst = xEst / norm * earthRadius
    yEst = yEst / norm * earthRadius
    zEst = zEst / norm * earthRadius

    lat, lon := cartesianToGeo(xEst, yEst, zEst)

    return Location{Lat: lat, Lon: lon}
}

func multilateralTriangulation(results []Result, numServers int) Location {
    if len(results) < 3 {
        return Location{Lat: 0, Lon: 0}
    }

    if numServers > len(results) {
        numServers = len(results)
    }

    var totalLat, totalLon, totalWeight float64

    for i := 0; i < numServers; i++ {
        // Poids inversement proportionnel au delta
        weight := 1.0 / (float64(results[i].Delta.Milliseconds()) + 1.0)
        
        totalLat += results[i].Server.Lat * weight
        totalLon += results[i].Server.Lon * weight
        totalWeight += weight
    }

    return Location{
        Lat: totalLat / totalWeight,
        Lon: totalLon / totalWeight,
    }
}

func getServerDatabase() []Server {
    return []Server{
        // === EUROPE ===
        
        // FRANCE (8 serveurs)
        {"Cloudflare", "1.1.1.1", "France", "Paris", 48.8566, 2.3522, 0},
        {"Google DNS", "216.58.213.195", "France", "Paris", 48.8566, 2.3522, 0},
        {"OVH", "54.36.0.1", "France", "Paris", 48.8566, 2.3522, 0},
        {"Scaleway", "51.15.0.1", "France", "Paris", 48.8566, 2.3522, 0},
        {"Online", "62.210.0.1", "France", "Paris", 48.8566, 2.3522, 0},
        {"Free", "212.27.48.10", "France", "Paris", 48.8566, 2.3522, 0},
        {"Orange", "80.10.246.2", "France", "Paris", 48.8566, 2.3522, 0},
        {"OVH-Strasbourg", "51.68.0.1", "France", "Strasbourg", 48.5734, 7.7521, 0},

        // ROYAUME-UNI (7 serveurs)
        {"Google-UK", "8.8.4.4", "UK", "London", 51.5074, -0.1278, 0},
        {"Cloudflare-UK", "1.0.0.1", "UK", "London", 51.5074, -0.1278, 0},
        {"BBC", "212.58.244.67", "UK", "London", 51.5074, -0.1278, 0},
        {"DigitalOcean", "178.62.0.1", "UK", "London", 51.5074, -0.1278, 0},
        {"Linode", "178.79.128.1", "UK", "London", 51.5074, -0.1278, 0},
        {"Vodafone", "194.73.73.73", "UK", "London", 51.5074, -0.1278, 0},
        {"BT", "194.72.9.38", "UK", "London", 51.5074, -0.1278, 0},

        // ALLEMAGNE (8 serveurs)
        {"Hetzner", "213.133.100.1", "Germany", "Frankfurt", 50.1109, 8.6821, 0},
        {"AWS-DE", "52.59.0.1", "Germany", "Frankfurt", 50.1109, 8.6821, 0},
        {"Google-DE", "216.58.207.67", "Germany", "Frankfurt", 50.1109, 8.6821, 0},
        {"Contabo", "213.136.64.1", "Germany", "Frankfurt", 50.1109, 8.6821, 0},
        {"IONOS", "217.160.0.1", "Germany", "Frankfurt", 50.1109, 8.6821, 0},
        {"Telekom-DE", "217.0.43.145", "Germany", "Frankfurt", 50.1109, 8.6821, 0},
        {"Hetzner-Nuremberg", "213.239.192.1", "Germany", "Nuremberg", 49.4521, 11.0767, 0},
        {"1&1", "217.237.148.22", "Germany", "Karlsruhe", 49.0069, 8.4037, 0},

        // PAYS-BAS (6 serveurs)
        {"Transip", "195.8.195.8", "Netherlands", "Amsterdam", 52.3676, 4.9041, 0},
        {"LeaseWeb", "5.79.73.204", "Netherlands", "Amsterdam", 52.3676, 4.9041, 0},
        {"Vultr-AMS", "108.61.0.1", "Netherlands", "Amsterdam", 52.3676, 4.9041, 0},
        {"DigitalOcean-AMS", "188.166.0.1", "Netherlands", "Amsterdam", 52.3676, 4.9041, 0},
        {"Google-NL", "216.58.211.3", "Netherlands", "Amsterdam", 52.3676, 4.9041, 0},
        {"KPN", "195.121.1.34", "Netherlands", "Rotterdam", 51.9225, 4.4792, 0},

        // ESPAGNE (5 serveurs)
        {"Telefonica", "194.179.1.100", "Spain", "Madrid", 40.4168, -3.7038, 0},
        {"Orange-ES", "62.36.225.150", "Spain", "Madrid", 40.4168, -3.7038, 0},
        {"Vodafone-ES", "193.110.157.151", "Spain", "Madrid", 40.4168, -3.7038, 0},
        {"AWS-ES", "15.161.0.1", "Spain", "Madrid", 40.4168, -3.7038, 0},
        {"Google-ES", "216.58.215.67", "Spain", "Barcelona", 41.3851, 2.1734, 0},

        // ITALIE (5 serveurs)
        {"Aruba", "62.149.128.2", "Italy", "Milan", 45.4642, 9.1900, 0},
        {"Telecom-IT", "151.99.125.1", "Italy", "Milan", 45.4642, 9.1900, 0},
        {"Fastweb", "195.110.124.188", "Italy", "Milan", 45.4642, 9.1900, 0},
        {"Google-IT", "216.58.213.3", "Italy", "Milan", 45.4642, 9.1900, 0},
        {"AWS-IT", "15.160.0.1", "Italy", "Milan", 45.4642, 9.1900, 0},

        // SUISSE (5 serveurs)
        {"Swisscom", "195.186.1.111", "Switzerland", "Zurich", 47.3769, 8.5417, 0},
        {"Init7", "77.109.128.2", "Switzerland", "Zurich", 47.3769, 8.5417, 0},
        {"Google-CH", "216.58.215.3", "Switzerland", "Zurich", 47.3769, 8.5417, 0},
        {"Cloudflare-CH", "162.158.0.1", "Switzerland", "Geneva", 46.2044, 6.1432, 0},
        {"Green", "80.74.140.10", "Switzerland", "Zurich", 47.3769, 8.5417, 0},

        // SUÈDE (5 serveurs)
        {"Telia-SE", "62.20.66.66", "Sweden", "Stockholm", 59.3293, 18.0686, 0},
        {"Bahnhof", "195.67.199.2", "Sweden", "Stockholm", 59.3293, 18.0686, 0},
        {"Google-SE", "216.58.211.67", "Sweden", "Stockholm", 59.3293, 18.0686, 0},
        {"AWS-SE", "13.48.0.1", "Sweden", "Stockholm", 59.3293, 18.0686, 0},
        {"TeliaSonera", "213.242.116.19", "Sweden", "Stockholm", 59.3293, 18.0686, 0},

        // POLOGNE (5 serveurs)
        {"OVH-PL", "91.216.107.2", "Poland", "Warsaw", 52.2297, 21.0122, 0},
        {"Google-PL", "216.58.215.195", "Poland", "Warsaw", 52.2297, 21.0122, 0},
        {"Orange-PL", "80.55.240.10", "Poland", "Warsaw", 52.2297, 21.0122, 0},
        {"T-Mobile-PL", "213.180.130.10", "Poland", "Warsaw", 52.2297, 21.0122, 0},
        {"AWS-PL", "15.236.0.1", "Poland", "Warsaw", 52.2297, 21.0122, 0},

        // USA - EST (New York) (7 serveurs)
        {"Google-NY", "142.250.185.46", "USA", "New York", 40.7128, -74.0060, 0},
        {"DigitalOcean-NY", "192.241.128.1", "USA", "New York", 40.7128, -74.0060, 0},
        {"Linode-Newark", "66.228.32.1", "USA", "Newark", 40.7357, -74.1724, 0},
        {"Verizon-NY", "208.48.0.1", "USA", "New York", 40.7128, -74.0060, 0},
        {"GTT-NY", "89.149.128.1", "USA", "New York", 40.7128, -74.0060, 0},
        {"AWS-NY", "54.210.0.1", "USA", "New York", 40.7128, -74.0060, 0},
        {"Hurricane-NY", "216.66.1.2", "USA", "New York", 40.7128, -74.0060, 0},

        // USA - OUEST (Californie) (7 serveurs)
        {"Google-CA", "216.58.217.206", "USA", "Los Angeles", 34.0522, -118.2437, 0},
        {"Cloudflare-SJ", "104.16.0.1", "USA", "San Jose", 37.3382, -121.8863, 0},
        {"AWS-CA", "52.8.0.1", "USA", "San Francisco", 37.7749, -122.4194, 0},
        {"DigitalOcean-SF", "159.65.0.1", "USA", "San Francisco", 37.7749, -122.4194, 0},
        {"Linode-Fremont", "50.116.0.1", "USA", "Fremont", 37.5483, -121.9886, 0},
        {"Hurricane-LA", "216.218.186.2", "USA", "Los Angeles", 34.0522, -118.2437, 0},
        {"Cogent-LA", "38.142.0.1", "USA", "Los Angeles", 34.0522, -118.2437, 0},

        // USA - CENTRE (Chicago) (5 serveurs)
        {"Vultr-Chicago", "207.246.64.1", "USA", "Chicago", 41.8781, -87.6298, 0},
        {"DigitalOcean-CHI", "159.89.0.1", "USA", "Chicago", 41.8781, -87.6298, 0},
        {"Google-CHI", "216.58.193.46", "USA", "Chicago", 41.8781, -87.6298, 0},
        {"AWS-CHI", "3.128.0.1", "USA", "Chicago", 41.8781, -87.6298, 0},
        {"Linode-Chicago", "45.79.0.1", "USA", "Chicago", 41.8781, -87.6298, 0},

        // USA - SUD (Texas) (5 serveurs)
        {"Google-TX", "216.58.195.46", "USA", "Dallas", 32.7767, -96.7970, 0},
        {"Vultr-Dallas", "108.61.224.1", "USA", "Dallas", 32.7767, -96.7970, 0},
        {"AWS-TX", "3.16.0.1", "USA", "Dallas", 32.7767, -96.7970, 0},
        {"DigitalOcean-TX", "159.203.0.1", "USA", "Dallas", 32.7767, -96.7970, 0},
        {"Hurricane-TX", "64.62.128.1", "USA", "Dallas", 32.7767, -96.7970, 0},

        // CANADA (6 serveurs)
        {"OVH-CA", "51.222.0.1", "Canada", "Montreal", 45.5017, -73.5673, 0},
        {"Google-CA", "216.58.193.67", "Canada", "Toronto", 43.6532, -79.3832, 0},
        {"AWS-CA", "15.223.0.1", "Canada", "Montreal", 45.5017, -73.5673, 0},
        {"DigitalOcean-TOR", "159.203.64.1", "Canada", "Toronto", 43.6532, -79.3832, 0},
        {"Cloudflare-TOR", "104.16.128.1", "Canada", "Toronto", 43.6532, -79.3832, 0},
        {"Bell-CA", "64.230.160.1", "Canada", "Montreal", 45.5017, -73.5673, 0},

        // BRÉSIL (6 serveurs)
        {"Google-BR", "216.58.222.67", "Brazil", "São Paulo", -23.5505, -46.6333, 0},
        {"AWS-BR", "18.231.0.1", "Brazil", "São Paulo", -23.5505, -46.6333, 0},
        {"Cloudflare-BR", "104.16.192.1", "Brazil", "São Paulo", -23.5505, -46.6333, 0},
        {"DigitalOcean-BR", "159.89.192.1", "Brazil", "São Paulo", -23.5505, -46.6333, 0},
        {"Locaweb", "200.234.224.2", "Brazil", "São Paulo", -23.5505, -46.6333, 0},
        {"Vivo-BR", "200.142.0.1", "Brazil", "Rio de Janeiro", -22.9068, -43.1729, 0},

        // ARGENTINE (5 serveurs)
        {"Google-AR", "216.58.222.195", "Argentina", "Buenos Aires", -34.6037, -58.3816, 0},
        {"Telecom-AR", "200.51.211.11", "Argentina", "Buenos Aires", -34.6037, -58.3816, 0},
        {"Claro-AR", "200.45.191.11", "Argentina", "Buenos Aires", -34.6037, -58.3816, 0},
        {"Arsat", "200.61.47.1", "Argentina", "Buenos Aires", -34.6037, -58.3816, 0},
        {"Fibertel", "200.115.100.2", "Argentina", "Buenos Aires", -34.6037, -58.3816, 0},

        // CHILI (5 serveurs)
        {"Google-CL", "216.58.222.3", "Chile", "Santiago", -33.4489, -70.6693, 0},
        {"AWS-CL", "15.220.0.1", "Chile", "Santiago", -33.4489, -70.6693, 0},
        {"Movistar-CL", "200.28.16.68", "Chile", "Santiago", -33.4489, -70.6693, 0},
        {"VTR", "200.104.237.131", "Chile", "Santiago", -33.4489, -70.6693, 0},
        {"Entel-CL", "200.73.97.18", "Chile", "Santiago", -33.4489, -70.6693, 0},

        // JAPON (7 serveurs)
        {"Google-JP", "216.58.220.195", "Japan", "Tokyo", 35.6762, 139.6503, 0},
        {"AWS-JP", "54.178.0.1", "Japan", "Tokyo", 35.6762, 139.6503, 0},
        {"Linode-JP", "139.162.64.1", "Japan", "Tokyo", 35.6762, 139.6503, 0},
        {"Sakura", "153.120.0.1", "Japan", "Tokyo", 35.6762, 139.6503, 0},
        {"GMO", "157.7.0.1", "Japan", "Tokyo", 35.6762, 139.6503, 0},
        {"NTT-JP", "129.250.0.1", "Japan", "Tokyo", 35.6762, 139.6503, 0},
        {"Softbank", "221.113.192.1", "Japan", "Tokyo", 35.6762, 139.6503, 0},

        // SINGAPOUR (6 serveurs)
        {"Google-SG", "216.58.199.67", "Singapore", "Singapore", 1.3521, 103.8198, 0},
        {"AWS-SG", "54.254.0.1", "Singapore", "Singapore", 1.3521, 103.8198, 0},
        {"DigitalOcean-SG", "188.166.128.1", "Singapore", "Singapore", 1.3521, 103.8198, 0},
        {"Linode-SG", "139.162.0.1", "Singapore", "Singapore", 1.3521, 103.8198, 0},
        {"Vultr-SG", "45.32.0.1", "Singapore", "Singapore", 1.3521, 103.8198, 0},
        {"Singtel", "165.21.0.1", "Singapore", "Singapore", 1.3521, 103.8198, 0},

        // CORÉE DU SUD (5 serveurs)
        {"Google-KR", "216.58.197.67", "South Korea", "Seoul", 37.5665, 126.9780, 0},
        {"AWS-KR", "3.36.0.1", "South Korea", "Seoul", 37.5665, 126.9780, 0},
        {"KT", "168.126.63.1", "South Korea", "Seoul", 37.5665, 126.9780, 0},
        {"LG-U+", "164.124.101.2", "South Korea", "Seoul", 37.5665, 126.9780, 0},
        {"SK-Telecom", "210.220.163.82", "South Korea", "Seoul", 37.5665, 126.9780, 0},

        // INDE (6 serveurs)
        {"Google-IN", "216.58.196.67", "India", "Mumbai", 19.0760, 72.8777, 0},
        {"AWS-IN", "13.233.0.1", "India", "Mumbai", 19.0760, 72.8777, 0},
        {"DigitalOcean-IN", "159.65.144.1", "India", "Bangalore", 12.9716, 77.5946, 0},
        {"Cloudflare-IN", "104.16.224.1", "India", "Mumbai", 19.0760, 72.8777, 0},
        {"Bharti", "182.74.0.1", "India", "Delhi", 28.7041, 77.1025, 0},
        {"Reliance", "49.205.0.1", "India", "Mumbai", 19.0760, 72.8777, 0},

        // HONG KONG (5 serveurs)
        {"Google-HK", "216.58.197.195", "Hong Kong", "Hong Kong", 22.3193, 114.1694, 0},
        {"AWS-HK", "18.166.0.1", "Hong Kong", "Hong Kong", 22.3193, 114.1694, 0},
        {"DigitalOcean-HK", "159.89.224.1", "Hong Kong", "Hong Kong", 22.3193, 114.1694, 0},
        {"Cloudflare-HK", "104.16.64.1", "Hong Kong", "Hong Kong", 22.3193, 114.1694, 0},
        {"PCCW", "202.45.128.1", "Hong Kong", "Hong Kong", 22.3193, 114.1694, 0},

        // AUSTRALIE (7 serveurs)
        {"Google-AU", "216.58.203.67", "Australia", "Sydney", -33.8688, 151.2093, 0},
        {"AWS-AU", "54.206.0.1", "Australia", "Sydney", -33.8688, 151.2093, 0},
        {"DigitalOcean-AU", "159.65.128.1", "Australia", "Sydney", -33.8688, 151.2093, 0},
        {"Linode-AU", "172.105.160.1", "Australia", "Sydney", -33.8688, 151.2093, 0},
        {"Vultr-AU", "45.76.0.1", "Australia", "Sydney", -33.8688, 151.2093, 0},
        {"Telstra", "203.50.0.1", "Australia", "Melbourne", -37.8136, 144.9631, 0},
        {"Optus", "211.29.132.12", "Australia", "Sydney", -33.8688, 151.2093, 0},

        // NOUVELLE-ZÉLANDE (5 serveurs)
        {"Google-NZ", "216.58.199.195", "New Zealand", "Auckland", -36.8485, 174.7633, 0},
        {"AWS-NZ", "13.239.0.1", "New Zealand", "Auckland", -36.8485, 174.7633, 0},
        {"Spark", "203.109.129.68", "New Zealand", "Auckland", -36.8485, 174.7633, 0},
        {"Vodafone-NZ", "202.27.184.3", "New Zealand", "Auckland", -36.8485, 174.7633, 0},
        {"2degrees", "203.167.251.1", "New Zealand", "Auckland", -36.8485, 174.7633, 0},

        // AFRIQUE DU SUD (6 serveurs)
        {"Google-ZA", "216.58.223.67", "South Africa", "Johannesburg", -26.2041, 28.0473, 0},
        {"AWS-ZA", "13.244.0.1", "South Africa", "Cape Town", -33.9249, 18.4241, 0},
        {"Cloudflare-ZA", "104.17.0.1", "South Africa", "Johannesburg", -26.2041, 28.0473, 0},
        {"Telkom", "196.25.1.1", "South Africa", "Johannesburg", -26.2041, 28.0473, 0},
        {"MTN", "41.203.0.1", "South Africa", "Johannesburg", -26.2041, 28.0473, 0},
        {"Vodacom", "196.207.40.165", "South Africa", "Johannesburg", -26.2041, 28.0473, 0},

        // ÉGYPTE (5 serveurs)
        {"Google-EG", "216.58.214.195", "Egypt", "Cairo", 30.0444, 31.2357, 0},
        {"Cloudflare-EG", "104.17.64.1", "Egypt", "Cairo", 30.0444, 31.2357, 0},
        {"TE-Data", "196.219.0.1", "Egypt", "Cairo", 30.0444, 31.2357, 0},
        {"Orange-EG", "41.128.0.1", "Egypt", "Cairo", 30.0444, 31.2357, 0},
        {"Vodafone-EG", "41.32.0.1", "Egypt", "Cairo", 30.0444, 31.2357, 0},

        // ÉMIRATS ARABES UNIS (5 serveurs)
        {"Google-UAE", "216.58.214.67", "UAE", "Dubai", 25.2048, 55.2708, 0},
        {"AWS-UAE", "3.29.0.1", "UAE", "Dubai", 25.2048, 55.2708, 0},
        {"Cloudflare-UAE", "104.17.128.1", "UAE", "Dubai", 25.2048, 55.2708, 0},
        {"Etisalat", "213.42.20.20", "UAE", "Dubai", 25.2048, 55.2708, 0},
        {"Du", "195.229.241.222", "UAE", "Dubai", 25.2048, 55.2708, 0},

        // ISRAËL (5 serveurs)
        {"Google-IL", "216.58.212.195", "Israel", "Tel Aviv", 32.0853, 34.7818, 0},
        {"AWS-IL", "3.120.0.1", "Israel", "Tel Aviv", 32.0853, 34.7818, 0},
        {"Bezeq", "80.178.0.1", "Israel", "Tel Aviv", 32.0853, 34.7818, 0},
        {"Cellcom", "62.90.0.1", "Israel", "Tel Aviv", 32.0853, 34.7818, 0},
        {"HOT", "79.178.0.1", "Israel", "Tel Aviv", 32.0853, 34.7818, 0},

        // DNS PUBLICS GLOBAUX (référence)
        {"Google-DNS-1", "8.8.8.8", "Global", "USA", 37.4056, -122.0775, 0},
        {"Google-DNS-2", "8.8.4.4", "Global", "USA", 37.4056, -122.0775, 0},
        {"Quad9", "9.9.9.9", "Global", "USA", 37.7749, -122.4194, 0},
        {"OpenDNS-1", "208.67.222.222", "Global", "USA", 37.7749, -122.4194, 0},
        {"OpenDNS-2", "208.67.220.220", "Global", "USA", 37.7749, -122.4194, 0},
    }
}


func getUserInput() string {
    reader := bufio.NewReader(os.Stdin)
    
    fmt.Println("\n" + strings.Repeat("=", 63))
    fmt.Println("       SYSTEME DE TRIANGULATION IP PAR LATENCE")
    fmt.Println(strings.Repeat("=", 63))
    
    fmt.Print("\nEntrez l'IP ou domaine cible : ")
    
    input, _ := reader.ReadString('\n')
    input = strings.TrimSpace(input)
    
    if input == "" {
        fmt.Println("\nErreur: Aucune IP fournie")
        os.Exit(1)
    }
    
    return input
}

func displayResults(results []Result, targetIP string, targetRTT time.Duration) {
    fmt.Println("\n" + strings.Repeat("=", 80))
    fmt.Printf("RESULTATS DE L'ANALYSE - Cible: %s (RTT: %v)\n", targetIP, targetRTT)
    fmt.Println(strings.Repeat("=", 80))

    fmt.Println("\nTOP 15 SERVEURS LES PLUS PROCHES (par similarité de latence)")
    fmt.Println(strings.Repeat("-", 80))
    
    for i := 0; i < 15 && i < len(results); i++ {
        r := results[i]
        
        // Indicateur de proximité
        proximity := "[+++]"
        if r.Delta > 50*time.Millisecond {
            proximity = "[++ ]"
        }
        if r.Delta > 100*time.Millisecond {
            proximity = "[+  ]"
        }
        if r.Delta > 200*time.Millisecond {
            proximity = "[   ]"
        }
        
        fmt.Printf("%s %2d) %-20s | %-15s | %-12s\n",
            proximity, i+1, r.Server.Name, r.Server.Country, r.Server.City)
        fmt.Printf("        RTT: %6v | Delta: %6v | Distance estimée: %.0f km\n",
            r.Server.AvgRTT, r.Delta, r.Distance)
        fmt.Println()
    }
}

func displayTriangulation(results []Result) {
    if len(results) < 3 {
        fmt.Println("\nErreur: Pas assez de serveurs pour la triangulation")
        return
    }

    fmt.Println("\n" + strings.Repeat("=", 80))
    fmt.Println("TRIANGULATION MATHEMATIQUE")
    fmt.Println(strings.Repeat("=", 80))

    // Méthode 1 : Trilatération simple (3 meilleurs serveurs)
    s1, s2, s3 := results[0].Server, results[1].Server, results[2].Server
    d1, d2, d3 := results[0].Distance, results[1].Distance, results[2].Distance

    loc1 := trilaterate(s1, s2, s3, d1, d2, d3)

    fmt.Println("\nMETHODE 1: Trilatération 3-points")
    fmt.Println(strings.Repeat("-", 80))
    fmt.Printf("Serveur 1: %s (%s) - Distance: %.0f km\n", s1.Name, s1.City, d1)
    fmt.Printf("Serveur 2: %s (%s) - Distance: %.0f km\n", s2.Name, s2.City, d2)
    fmt.Printf("Serveur 3: %s (%s) - Distance: %.0f km\n", s3.Name, s3.City, d3)
    fmt.Printf("\nPosition estimée: %.4f, %.4f\n", loc1.Lat, loc1.Lon)
    fmt.Printf("Google Maps: https://www.google.com/maps?q=%.4f,%.4f\n", loc1.Lat, loc1.Lon)

    // Méthode 2 : Multilatération (10 meilleurs serveurs)
    numServers := 10
    if len(results) < numServers {
        numServers = len(results)
    }
    
    loc2 := multilateralTriangulation(results, numServers)

    fmt.Println("\nMETHODE 2: Multilatération pondérée (top " + fmt.Sprint(numServers) + " serveurs)")
    fmt.Println(strings.Repeat("-", 80))
    fmt.Printf("Position estimée: %.4f, %.4f\n", loc2.Lat, loc2.Lon)
    fmt.Printf("Google Maps: https://www.google.com/maps?q=%.4f,%.4f\n", loc2.Lat, loc2.Lon)

    // Visualisation ASCII du triangle
    fmt.Println("\nVISUALISATION DU TRIANGLE DE TRIANGULATION")
    fmt.Println(strings.Repeat("-", 80))
    fmt.Printf("\n              %s\n", s1.Name)
    fmt.Println("                /  \\")
    fmt.Println("               /    \\")
    fmt.Printf("          %.0f km    %.0f km\n", d1, 
        distance(s1.Lat, s1.Lon, loc1.Lat, loc1.Lon))
    fmt.Println("             /        \\")
    fmt.Println("            /   [*]    \\")
    fmt.Println("           /   CIBLE    \\")
    fmt.Println("          /              \\")
    fmt.Printf("    %s ----------- %s\n", s2.Name, s3.Name)
    fmt.Printf("               %.0f km\n", distance(s2.Lat, s2.Lon, s3.Lat, s3.Lon))

    // Distances géographiques entre serveurs
    fmt.Println("\nDISTANCES GEOGRAPHIQUES ENTRE SERVEURS")
    fmt.Println(strings.Repeat("-", 80))
    fmt.Printf("%s <-> %s: %.0f km\n", s1.Name, s2.Name, distance(s1.Lat, s1.Lon, s2.Lat, s2.Lon))
    fmt.Printf("%s <-> %s: %.0f km\n", s1.Name, s3.Name, distance(s1.Lat, s1.Lon, s3.Lat, s3.Lon))
    fmt.Printf("%s <-> %s: %.0f km\n", s2.Name, s3.Name, distance(s2.Lat, s2.Lon, s3.Lat, s3.Lon))

    // Analyse de cohérence
    fmt.Println("\nANALYSE DE COHERENCE")
    fmt.Println(strings.Repeat("-", 80))
    
    avgDelta := time.Duration(0)
    for i := 0; i < 5 && i < len(results); i++ {
        avgDelta += results[i].Delta
    }
    avgDelta /= time.Duration(5)
    
    coherence := "EXCELLENTE"
    if avgDelta > 50*time.Millisecond {
        coherence = "BONNE"
    }
    if avgDelta > 100*time.Millisecond {
        coherence = "MOYENNE"
    }
    if avgDelta > 200*time.Millisecond {
        coherence = "FAIBLE"
    }
    
    fmt.Printf("Cohérence de la triangulation: %s\n", coherence)
    fmt.Printf("Delta moyen (top 5): %v\n", avgDelta)
    fmt.Printf("Nombre de serveurs analysés: %d\n", len(results))

    // Estimation de la précision
    precision := 500.0 // km par défaut
    if avgDelta < 20*time.Millisecond {
        precision = 100.0
    } else if avgDelta < 50*time.Millisecond {
        precision = 200.0
    } else if avgDelta < 100*time.Millisecond {
        precision = 300.0
    }
    
    fmt.Printf("Précision estimée: +/- %.0f km\n", precision)
}


func displayStatistics(results []Result) {
    if len(results) == 0 {
        return
    }

    fmt.Println("\n" + strings.Repeat("=", 80))
    fmt.Println("STATISTIQUES GLOBALES")
    fmt.Println(strings.Repeat("=", 80))

    // Regroupement par pays
    countryStats := make(map[string]int)
    for _, r := range results {
        countryStats[r.Server.Country]++
    }

    fmt.Println("\nRépartition par pays (top 10):")
    
    type countryCount struct {
        country string
        count   int
    }
    
    var countries []countryCount
    for country, count := range countryStats {
        countries = append(countries, countryCount{country, count})
    }
    
    sort.Slice(countries, func(i, j int) bool {
        return countries[i].count > countries[j].count
    })
    
    for i := 0; i < 10 && i < len(countries); i++ {
        bar := strings.Repeat("#", countries[i].count)
        fmt.Printf("  %-20s %s %d\n", countries[i].country, bar, countries[i].count)
    }

    // RTT moyen
    var totalRTT time.Duration
    for _, r := range results {
        totalRTT += r.Server.AvgRTT
    }
    avgRTT := totalRTT / time.Duration(len(results))
    
    fmt.Printf("\nRTT moyen de tous les serveurs: %v\n", avgRTT)
    fmt.Printf("Nombre total de serveurs testés: %d\n", len(results))
}


func main() {
    targetIP := getUserInput()

    servers := getServerDatabase()
    
    targetRTT, err := AvgPing(targetIP, 5)
    if err != nil {
        fmt.Printf("\nErreur lors du ping de la cible: %v\n", err)
        fmt.Println("\nVerifiez que:")
        fmt.Println("   - L'IP/domaine est valide")
        fmt.Println("   - Vous avez les droits root (sudo)")
        fmt.Println("   - Le firewall autorise ICMP")
        return
    }

    fmt.Printf("RTT cible : %v\n\n", targetRTT)

    // Ping parallèle des serveurs
    fmt.Println("[+] Analyse des serveurs de référence (cela peut prendre 1-2 minutes)...")
    fmt.Println(strings.Repeat("-", 80))
    
    var wg sync.WaitGroup
    var mu sync.Mutex
    var results []Result
    
    progressCount := 0
    totalServers := len(servers)

    for _, s := range servers {
        wg.Add(1)
        go func(server Server) {
            defer wg.Done()
            
            avg, err := AvgPing(server.IP, 3)
            if err != nil {
                mu.Lock()
                progressCount++
                fmt.Printf("\r[%3d/%3d] [X] %s: erreur", progressCount, totalServers, server.Name)
                mu.Unlock()
                return
            }

            server.AvgRTT = avg
            delta := avg - targetRTT
            if delta < 0 {
                delta = -delta
            }

            // Calculer la distance estimée basée sur RTT
            estimatedDistance := rttToDistance(delta)

            mu.Lock()
            results = append(results, Result{
                Server:   server,
                Delta:    delta,
                Distance: estimatedDistance,
            })
            progressCount++
            fmt.Printf("\r[%3d/%3d] [OK] %s: %v", progressCount, totalServers, server.Name, avg)
            mu.Unlock()
        }(s)
        
        // délai pour éviter de surcharger(bug une fois sur deux...)
        time.Sleep(10 * time.Millisecond)
    }

    wg.Wait()
    fmt.Println("\n")

    if len(results) == 0 {
        fmt.Println("\nErreur: Aucun serveur n'a répondu. Vérifiez votre connexion.")
        return
    }

    // Tri par delta
    sort.Slice(results, func(i, j int) bool {
        return results[i].Delta < results[j].Delta
    })

    // Affichage des résultats
    displayResults(results, targetIP, targetRTT)
    displayTriangulation(results)
    displayStatistics(results)

    fmt.Println("\n" + strings.Repeat("=", 80))
    fmt.Println("ANALYSE TERMINEE")
    fmt.Println(strings.Repeat("=", 80))
}
