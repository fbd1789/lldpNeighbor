package main

import (
	"fmt"
	"sync"
	"strings"
	"sort"
	"os"
	"strconv"
	"github.com/aristanetworks/goeapi"
	"github.com/xuri/excelize/v2"
	"gopkg.in/ini.v1"
)

// Conn structure holds the connection information for an EOS device
type Conn struct {
	Transport string
	Host      string
	Username  string
	Password  string
	Port      int
	Config    string
}

// Simple function that returns the show lldp neighbors string.
func (s *ShLldpNeighbor) GetCmd() string {
	return "show lldp neighbors"
}

// ------------------------------show lldp neighbor --------------------
type LLDPNeighbor struct {
	Machine        string `json:"-"`
	Port           string `json:"port"`
	NeighborDevice string `json:"neighborDevice"`
	NeighborPort   string `json:"neighborPort"`
}

type ShLldpNeighbor struct {
	LLDPNeighbors []LLDPNeighbor `json:"lldpNeighbors"`
}

func (c *Conn) Connect() (*goeapi.Node, error) {
	connect, err := goeapi.Connect(c.Transport, c.Host, c.Username, c.Password, c.Port)
	if err != nil {
		fmt.Println(err)
	}
	return connect, nil
}


func main() {
	// Read the config.ini file
	inidata, err := ini.Load("config.ini")
	if err != nil {
	   fmt.Printf("Fail to read file: %v", err)
	   os.Exit(1)
	 }
	UserName := inidata.Section("global").Key("username").String()
	Password := inidata.Section("global").Key("password").String()
	Transport := inidata.Section("transport").Key("transport").String()
	// Port := inidata.Section("transport").Key("port").String()
	Port, err := strconv.Atoi( inidata.Section("transport").Key("port").String())
    if err != nil {
        fmt.Printf("Erreur lors de la conversion en entier: %v", err)
    }
	Hosts := inidata.Section("global").Key("devices").String()

	// Séparer la chaîne en un tableau de chaînes, en utilisant la virgule comme délimiteur
    hosts := strings.Split(Hosts, ",")
	// Supprimer les espaces autour de chaque élément
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}
	// Liste des hôtes à interroger
	// hosts := []string{"Spine1", "Spine2","Spine3", "Leaf1a","Leaf1b", "Leaf2a","Leaf2b"}
	

	// Liste pour stocker les résultats de chaque hôte (thread-safe)
	var allNeighbors []ShLldpNeighbor
	var mu sync.Mutex // Pour synchroniser l'accès à allNeighbors

	// WaitGroup pour attendre la fin de toutes les goroutines
	var wg sync.WaitGroup

	// Fonction pour interroger un hôte
	queryHost := func(host string) {
		defer wg.Done() // Signaler la fin de la goroutine

		fmt.Printf("Connexion à l'hôte %s...\n", host)

		d := Conn{
			Transport: Transport,
			Host:      host,
			Username:  UserName,
			Password:  Password,
			Port:      Port,
		}

		Connect, err := d.Connect()
		if err != nil {
			fmt.Printf("Erreur de connexion à l'hôte %s: %v\n", host, err)
			return
		}

		shLldpNeighbor := &ShLldpNeighbor{}

		handle, err := Connect.GetHandle("json")
		if err != nil {
			fmt.Printf("Erreur lors de la récupération du handle pour l'hôte %s: %v\n", host, err)
			return
		}

		handle.AddCommand(shLldpNeighbor)
		if err := handle.Call(); err != nil {
			fmt.Printf("Erreur lors de l'exécution de la commande sur l'hôte %s: %v\n", host, err)
			return
		}

		for i := range shLldpNeighbor.LLDPNeighbors {
			shLldpNeighbor.LLDPNeighbors[i].Machine = d.Host
		}

		// Protection lors de l'ajout des résultats à allNeighbors
		mu.Lock()
		allNeighbors = append(allNeighbors, *shLldpNeighbor)
		mu.Unlock()
	}

	// Lancer une goroutine pour chaque hôte
	for _, host := range hosts {
		wg.Add(1)     // Incrémenter le compteur du WaitGroup
		go queryHost(host) // Lancer la requête en parallèle
	}

	// Attendre la fin de toutes les goroutines
	wg.Wait()

	// Fusion des donnees 
	var fusionAllNeighbors []string
	uniqueNeighbors := make(map[string]bool)
	for _, shLldpNeighbor := range allNeighbors {
		for _, neighbor := range shLldpNeighbor.LLDPNeighbors {
			// Vérifier si "Management0" est présent dans Port ou NeighborPort
			if strings.Contains(neighbor.Port, "Management0") || strings.Contains(neighbor.NeighborPort, "Management0") {
				continue // Ignorer cet enregistrement
			}
			// Créer une chaîne de caractères pour chaque voisin
			// entry := fmt.Sprintf("%s %s %s %s", neighbor.Machine, neighbor.Port, neighbor.NeighborDevice, neighbor.NeighborPort)
			
			// Créer une clé canonique en triant les éléments
			// Clé 1: dans l'ordre original
			key1 := fmt.Sprintf("%s %s %s %s", neighbor.Machine, neighbor.Port, neighbor.NeighborDevice, neighbor.NeighborPort)
			// Clé 2: dans l'ordre inversé
			key2 := fmt.Sprintf("%s %s %s %s", neighbor.NeighborDevice, neighbor.NeighborPort, neighbor.Machine, neighbor.Port)

			// Trier les clés pour s'assurer qu'elles sont identiques pour les paires inversées
			keys := []string{key1, key2}
			sort.Strings(keys)
			canonicalKey := strings.Join(keys, "|")

			// Ajouter seulement si la clé canonique n'a pas été vue
			if !uniqueNeighbors[canonicalKey] {
				uniqueNeighbors[canonicalKey] = true
				fusionAllNeighbors = append(fusionAllNeighbors, key1)
			}
			// fusionAllNeighbors = append(fusionAllNeighbors, entry)
		}
	}


	// Crée un nouveau fichier Excel
    f := excelize.NewFile()

    // Définit le nom de la feuille de calcul
    sheetName := "Sheet1"

	// Definition des titres de colonnes
	headers := []string{"Device 1", "Interface 1", "Device 2", "Interface 2"}

	// Insère les en-têtes de colonne
	for j, header := range headers {
		cellRef, _ := excelize.CoordinatesToCellName(j+1, 1) // Ligne 1 pour les en-têtes
		f.SetCellValue(sheetName, cellRef, header)
	}

    // Remplit la feuille de calcul avec les données
    for i, row := range fusionAllNeighbors {
		// Les donnees de fusioAllNeigbors sont des string, il faut remettre "Spine3 Ethernet4 Leaf2b Ethernet6" dans un tableau
		elements := strings.Fields(row)
		table := []string{
			elements[0], elements[1], elements[2], elements[3],
		}
		// Mise en place dans les cellules
        for j, cell := range table {
            cellRef, _ := excelize.CoordinatesToCellName(j+1, i+2)  // i+2 pour commencer à la ligne 2
            f.SetCellValue(sheetName, cellRef, cell)
        }
	}
	f.SaveAs("example.xlsx")


}