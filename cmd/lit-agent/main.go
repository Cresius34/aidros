package main

// Lit dans la base bolt d'une installation AJEAN ce que porte la clé "agent"
// du seau "state" : c'est ce que agentEnabled() lit.
//
// Mesure visee : l'amont 0.20.1 retourne le defaut du mode agent (eteint ->
// ACTIF). Si la cle est absente de notre base en service, le nouveau defaut
// s'appliquera des la bascule ; si elle porte "0" ou "1", le reglage existant
// gagne et le defaut ne change rien pour nous.

import (
	"fmt"
	"os"

	bolt "go.etcd.io/bbolt"
)

func main() {
	path := os.Args[1]
	db, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true, Timeout: 0})
	if err != nil {
		fmt.Println("ouverture impossible :", err)
		os.Exit(1)
	}
	defer db.Close()

	bucket := os.Args[2]
	if bucket == "" {
		bucket = "state"
	}

	_ = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			fmt.Printf("seau %q : ABSENT\n", bucket)
			return nil
		}
		fmt.Printf("seau %q : present\n", bucket)
		for _, k := range []string{"agent", "bridge_agent", "internet", "compact", "prewarm"} {
			v := b.Get([]byte(k))
			if v == nil {
				fmt.Printf("  cle %-14q : ABSENTE  <- le defaut du code s'applique\n", k)
				continue
			}
			fmt.Printf("  cle %-14q : %q\n", k, string(v))
		}
		// tout ce qui contient « agent », au cas ou la cle s'appellerait autrement
		fmt.Println("  --- toutes les cles contenant « agent » ---")
		n := 0
		b.ForEach(func(k, v []byte) error {
			if len(k) > 40 {
				return nil
			}
			ks := string(k)
			for i := 0; i+5 <= len(ks); i++ {
				if ks[i:i+5] == "agent" || ks[i:i+5] == "Agent" {
					vs := string(v)
					if len(vs) > 60 {
						vs = vs[:60] + "…"
					}
					fmt.Printf("    %s = %q\n", ks, vs)
					n++
					break
				}
			}
			return nil
		})
		if n == 0 {
			fmt.Println("    (aucune)")
		}
		return nil
	})
}
