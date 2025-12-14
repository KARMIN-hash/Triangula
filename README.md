# Triangula

Système de géolocalisation d'adresses IP par triangulation basée sur la latence réseau.

## Description

Ce programme utilise la mesure de latence (RTT - Round Trip Time) vers des serveurs pour estimer la position géographique d'une adresse IP cible.

## Prérequis

### Système d'exploitation
- Linux (recommandé)
- macOS
- Windows (avec limitations)

### Dépendances
```bash
go version >= 1.16
```

## Permissions

Le programme nécessite les privilèges root pour envoyer des paquets ICMP.

## Installation

### 1. Installer les dépendances Go
```bash
go get github.com/go-ping/ping
```
### 2. Compiler le programme
```bash
go build -o triangula main.go
```

## Algorithmes utilisés
### 1. Distance Haversine

Calcul de la distance géographique entre deux points sur une sphère :
```bash
d = 2R × arcsin(√(sin²(Δφ/2) + cos(φ1)×cos(φ2)×sin²(Δλ/2)))
```

### 2. Conversion RTT en distance
```bash
Distance = (RTT × vitesse_propagation) / 2
vitesse_propagation = vitesse_lumière × 0.67 (fibre optique)
```

### 3. Trilatération 3D

Conversion en coordonnées cartésiennes (ECEF), calcul du centre de gravité pondéré, reconversion en coordonnées géographiques.
### 4. Multilatération pondérée

Utilise les N meilleurs serveurs avec pondération et inversement proportionnelle au delta de latence :
```bash
Poids = 1 / (Delta + 1)
```
