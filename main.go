package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/NetchX/shadowsocks-multiuser/core"
)

var flags struct {
	ListCipher   bool
	DBHost       string
	DBPort       int
	DBUser       string
	DBPass       string
	DBName       string
	NodeID       int
	UDPEnabled   bool
	SyncInterval int
}

var startTime = time.Now().Unix()
var lastBandwidth uint64

func purge(instanceList map[int]*Instance, users []User) {
	for _, instance := range instanceList {
		contains := false

		for _, v := range users {
			if instance.Port == v.Port {
				contains = v.TransferEnable > v.Upload+v.Download
				break
			}
		}

		if !contains && instance.Started {
			instance.Stop()
		}
	}
}

func report(instance *Instance, database *Database) {
	log.Printf("Updating user %d uploaded %d downloaded %d to database", instance.UserID, instance.Bandwidth.Upload, instance.Bandwidth.Download)

	if err := database.UpdateBandwidth(instance); err == nil {
		lastBandwidth += instance.Bandwidth.Upload
		lastBandwidth += instance.Bandwidth.Download

		instance.Bandwidth.Reset()
	} else {
		log.Println(err)
	}
}

func update(instance *Instance, method, password string) {
	if instance.Method != method || instance.Password != password {
		instance.Method = method
		instance.Password = password

		restart(instance)
	}

	if instance.Started {
		restart(instance)
	}
}

func restart(instance *Instance) {
	if instance.Started {
		instance.Stop()
	}

	instance.Start()
}

func main() {
	flag.BoolVar(&flags.ListCipher, "listcipher", false, "List all cipher")
	flag.StringVar(&flags.DBHost, "dbhost", "localhost", "Database hostname")
	flag.IntVar(&flags.DBPort, "dbport", 3306, "Database port")
	flag.StringVar(&flags.DBUser, "dbuser", "root", "Database username")
	flag.StringVar(&flags.DBPass, "dbpass", "123456", "Database password")
	flag.StringVar(&flags.DBName, "dbname", "sspanel", "Database name")
	flag.IntVar(&flags.NodeID, "nodeid", -1, "Node ID")
	flag.IntVar(&flags.SyncInterval, "syncinterval", 30, "Sync interval")
	flag.BoolVar(&flags.UDPEnabled, "udp", false, "UDP forward")
	flag.Parse()

	if flags.ListCipher {
		for _, v := range core.ListCipher() {
			fmt.Println(v)
		}

		return
	}

	log.Println("Starting shadowsocks-multiuser")
	log.Println("Version: 1.0.0")

	if flags.NodeID == -1 {
		log.Println("Node id must be specified")
		return
	}

	instanceList := make(map[int]*Instance, 65535)
	first := true

	log.Println("Started")
	for {
		if !first {
			log.Printf("Wait %d seconds for sync users", flags.SyncInterval)
			time.Sleep(time.Second * time.Duration(flags.SyncInterval))
		} else {
			first = false
		}

		log.Println("Start syncing")

		log.Println("Opening database connection")
		database := newDatabase(flags.DBHost, flags.DBPort, flags.DBUser, flags.DBPass, flags.DBName, flags.NodeID)
		if err := database.Open(); err != nil {
			log.Println(err)
			continue
		}
		defer database.Close()

		log.Println("Get bandwidth total")
		total, err := database.GetNodeBandwidth()
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("Get bandwidth limit")
		limit, err := database.GetNodeBandwidthLimit()
		if err != nil {
			log.Println(err)
			continue
		}

		if total != 0 {
			if limit < total {
				purge(instanceList, make([]User, 0))
				for k := range instanceList {
					delete(instanceList, k)
				}

				log.Println("No more bandwidth left in this node")
			}
		}

		log.Println("Get traffic rate")
		rate, err := database.GetRate()
		if err != nil {
			log.Println(err)
			continue
		}
		database.NodeRate = rate

		log.Println("Update heartbeat")
		database.UpdateHeartbeat()

		log.Println("Get users")
		users, err := database.GetUser()
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("Purge users")
		purge(instanceList, users)

		for _, user := range users {
			log.Println(user)
			if instance, ok := instanceList[user.Port]; ok {
				if user.TransferEnable > user.Upload+user.Download {
					update(instance, user.Method, user.Password)
				} else {
					if instance.Started {
						instance.Stop()
					}

					report(instance, database)
					delete(instanceList, user.Port)
				}
			} else if user.TransferEnable > user.Upload+user.Download {
				log.Printf("Starting new instance for user %d", user.ID)
				instance := newInstance(user.ID, user.Port, user.Method, user.Password)
				instance.Start()

				instanceList[user.Port] = instance
			}
		}

		online := 0
		for _, instance := range instanceList {
			if time.Now().Unix()-instance.Bandwidth.Last < 10 {
				online++
				continue
			}

			if instance.Bandwidth.Upload != 0 || instance.Bandwidth.Download != 0 {
				report(instance, database)
			}
		}

		log.Println("Updating node bandwidth")
		err = database.UpdateNodeBandwidth()
		if err != nil {
			log.Println()
		} else {
			lastBandwidth = 0
		}

		log.Println("Updating node status")
		err = database.UpdateNodeStatus()
		if err != nil {
			log.Println(err)
		}

		log.Printf("Updating online users count: %d", online)
		err = database.UpdateOnlineUserCount(online)
		if err != nil {
			log.Println(err)
		}

		log.Println("Sync done")
	}
}
