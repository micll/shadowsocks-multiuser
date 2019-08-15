package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
)

// Database struct
type Database struct {
	Connection *sql.DB
	DBHost     string
	DBPort     int
	DBUser     string
	DBPass     string
	DBName     string
	NodeID     int
	NodeRate   float64
}

// User struct
type User struct {
	ID             int
	Upload         uint64
	Download       uint64
	Port           int
	Method         string
	Password       string
	Enable         int
	TransferEnable uint64
}

// Open database connection
func (database *Database) Open() error {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", database.DBUser, database.DBPass, database.DBHost, database.DBPort, database.DBName))
	if err != nil {
		return err
	}
	database.Connection = db

	return nil
}

// Close database connection
func (database *Database) Close() error {
	if database.Connection != nil {
		return database.Connection.Close()
	}

	return nil
}

// GetBandwidthRate R.T.
func (database *Database) GetBandwidthRate() (float64, error) {
	results, err := database.Connection.Query(fmt.Sprintf("SELECT traffic_rate FROM ss_node WHERE id=%d", database.NodeID))
	if err != nil {
		return -1, err
	}

	var rate float64
	if results.Next() {
		err = results.Scan(&rate)
		if err != nil {
			return -1, err
		}

		return rate, nil
	}

	return -1, errors.New("Node not found")
}

// GetUser R.T.
func (database *Database) GetUser() ([]User, error) {
	results, err := database.Connection.Query("SELECT id, u, d, port, method, passwd, enable, transfer_enable FROM user WHERE enable=1")
	if err != nil {
		return nil, err
	}

	users := make([]User, 65535)
	count := 0
	for results.Next() {
		var user User

		err = results.Scan(&user.ID, &user.Upload, &user.Download, &user.Port, &user.Method, &user.Password, &user.Enable, &user.TransferEnable)
		if err != nil {
			return nil, err
		}

		users[count] = user
		count++
	}

	return users[:count], nil
}

// GetNodeBandwidth R.T.
func (database *Database) GetNodeBandwidth() (uint64, error) {
	results, err := database.Connection.Query(fmt.Sprintf("SELECT node_bandwidth FROM ss_node WHERE id=%d", database.NodeID))
	if err != nil {
		return 0, err
	}

	var bandwidth uint64
	if results.Next() {
		err = results.Scan(&bandwidth)
		if err != nil {
			return 0, err
		}

		return bandwidth, err
	}

	return 0, errors.New("Node not found")
}

// GetNodeBandwidthLimit R.T.
func (database *Database) GetNodeBandwidthLimit() (uint64, error) {
	results, err := database.Connection.Query(fmt.Sprintf("SELECT node_bandwidth_limit FROM ss_node WHERE id=%d", database.NodeID))
	if err != nil {
		return 0, err
	}

	var bandwidth uint64
	if results.Next() {
		err = results.Scan(&bandwidth)
		if err != nil {
			return 0, err
		}

		return bandwidth, err
	}

	return 0, errors.New("Node not found")
}

// UpdateHeartbeat R.T.
func (database *Database) UpdateHeartbeat() error {
	_, err := database.Connection.Query(fmt.Sprintf("UPDATE `ss_node` SET node_heartbeat=%d", time.Now().Unix()))

	return err
}

// UpdateBandwidth R.T.
func (database *Database) UpdateBandwidth(instance *Instance) error {
	results, err := database.Connection.Query("SELECT u, d FROM `user`")
	if err != nil {
		return err
	}

	var cloudUpload uint64
	var cloudDownload uint64

	if results.Next() {
		err = results.Scan(&cloudUpload, &cloudDownload)
		if err != nil {
			return err
		}
	}

	userUpload := uint64(float64(instance.Bandwidth.Upload) * database.NodeRate)
	userDownload := uint64(float64(instance.Bandwidth.Download) * database.NodeRate)

	_, err = database.Connection.Query(fmt.Sprintf("INSERT INTO `user_traffic_log` (`user_id`, `u`, `d`, `node_id`, `rate`, `traffic`, `log_time`) VALUES (%d, %d, %d, %d, %f, %d, %d)", instance.UserID, userUpload, userDownload, database.NodeID, database.NodeRate, userUpload+userDownload, time.Now().Unix()))
	if err != nil {
		return err
	}

	cloudUpload += userUpload
	cloudDownload += userDownload
	_, err = database.Connection.Query(fmt.Sprintf("UPDATE user SET u=%d, d=%d, t=%d WHERE id=%d", cloudUpload, cloudDownload, time.Now().Unix(), instance.UserID))
	return err
}

// UpdateNodeBandwidth R.T.
func (database *Database) UpdateNodeBandwidth() error {
	bandwidth, err := database.GetNodeBandwidth()
	if err != nil {
		return err
	}
	bandwidth += lastBandwidth

	_, err = database.Connection.Query(fmt.Sprintf("UPDATE `ss_node` SET node_bandwidth=%d WHERE node_id=%d", bandwidth, database.NodeID))
	return err
}

// UpdateNodeStatus R.T.
func (database *Database) UpdateNodeStatus() error {
	uptime, err := host.Uptime()
	if err != nil {
		uptime = uint64(time.Now().Unix() - startTime)
	}

	var avgstr string
	avgstat, err := load.Avg()
	if err != nil {
		avgstr = "0.00 0.00 0.00"
	}
	avgstr = fmt.Sprintf("%.2f %.2f %.2f", avgstat.Load1, avgstat.Load5, avgstat.Load15)

	_, err = database.Connection.Query(fmt.Sprintf("INSERT INTO `ss_node_info` (`node_id`, `uptime`, `load`, `log_time`) VALUES (%d, %d, \"%s\", %d)", flags.NodeID, uptime, avgstr, time.Now().Unix()))

	return err
}

// UpdateOnlineUserCount R.T.
func (database *Database) UpdateOnlineUserCount(online int) error {
	_, err := database.Connection.Query(fmt.Sprintf("INSERT INTO `ss_node_online_log` (`node_id`, `online_user`, `log_time`) VALUES (%d, %d, %d)", flags.NodeID, online, time.Now().Unix()))

	return err
}

func newDatabase(host string, port int, user, pass, name string, id int) *Database {
	database := Database{}
	database.DBHost = host
	database.DBPort = port
	database.DBUser = user
	database.DBPass = pass
	database.DBName = name
	database.NodeID = id
	database.NodeRate = 1

	return &database
}
