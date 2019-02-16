// db_actions defines actions on the database
// Copyright (C) 2019 Emile Hansmaennel
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package db_actions

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"git.darknebu.la/GalaxySimulator/structs"
	_ "github.com/lib/pq"
	"io"
	"io/ioutil"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	DBUSER    = "postgres"
	DBNAME    = "postgres"
	DBSSLMODE = "disable"
)

var (
	db        *sql.DB
	treeWidth float64
)

// connectToDB returns a pointer to an sql database writing to the database
func ConnectToDB() *sql.DB {
	connStr := fmt.Sprintf("user=%s dbname=%s sslmode=%s", DBUSER, DBNAME, DBSSLMODE)
	db := dbConnect(connStr)
	return db
}

// dbConnect connects to a PostgreSQL database
func dbConnect(connStr string) *sql.DB {
	// connect to the database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("[ E ] connection: %v", err)
	}

	return db
}

// newTree creates a new tree with the given width
func NewTree(database *sql.DB, width float64) {
	db = database
	treeWidth = width

	log.Printf("Creating a new tree with a width of %f", width)

	// get the current max root id
	query := fmt.Sprintf("SELECT COALESCE(max(root_id), 0) FROM nodes")
	var currentMaxRootID int64
	err := db.QueryRow(query).Scan(&currentMaxRootID)
	if err != nil {
		log.Fatalf("[ E ] max root id query: %v\n\t\t\t query: %s\n", err, query)
	}

	// build the query creating a new node
	query = fmt.Sprintf("INSERT INTO nodes (box_width, root_id, box_center, depth, isleaf, timestep) VALUES (%f, %d, '{0, 0}', 0, TRUE, %d)", width, currentMaxRootID+1, currentMaxRootID+1)

	// execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] insert new node query: %v\n\t\t\t query: %s\n", err, query)
	}
}

// insertStar inserts the given star into the stars table and the nodes table tree
func InsertStar(database *sql.DB, star structs.Star2D, index int64) int64 {
	db = database
	start := time.Now()

	log.Printf("Inserting the star %v into the tree with the index %d", star, index)

	// insert the star into the stars table
	starID := insertIntoStars(star)

	// get the root node id
	query := fmt.Sprintf("select case when exists (select node_id from nodes where root_id=%d) then (select node_id from nodes where root_id=%d) else -1 end;", index, index)
	var id int64
	err := db.QueryRow(query).Scan(&id)

	// if there are no rows in the result set, create a new tree
	if err != nil {
		log.Fatalf("[ E ] Get root node id query: %v\n\t\t\t query: %s\n", err, query)
	}

	if id == -1 {
		NewTree(db, 1000)
		id = getRootNodeID(index)
	}

	log.Printf("Node id of the root node %d: %d", id, index)

	// insert the star into the tree (using it's ID) starting at the root
	insertIntoTree(starID, id)
	elapsedTime := time.Since(start)
	log.Printf("\t\t\t\t\t %s", elapsedTime)
	return starID
}

// insertIntoStars inserts the given star into the stars table
func insertIntoStars(star structs.Star2D) int64 {
	// unpack the star
	x := star.C.X
	y := star.C.Y
	vx := star.V.X
	vy := star.V.Y
	m := star.M

	// build the request query
	query := fmt.Sprintf("INSERT INTO stars (x, y, vx, vy, m) VALUES (%f, %f, %f, %f, %f) RETURNING star_id", x, y, vx, vy, m)

	// execute the query
	var starID int64
	err := db.QueryRow(query).Scan(&starID)
	if err != nil {
		log.Fatalf("[ E ] insert query: %v\n\t\t\t query: %s\n", err, query)
	}

	return starID
}

// insert into tree inserts the given star into the tree starting at the node with the given node id
func insertIntoTree(starID int64, nodeID int64) {
	//starRaw := getStar(starID)
	//nodeCenter := getBoxCenter(nodeID)
	//nodeWidth := getBoxWidth(nodeID)
	//log.Printf("[   ] \t Inserting star %v into the node (c: %v, w: %v)", starRaw, nodeCenter, nodeWidth)

	// There exist four cases:
	//                    | Contains a Star | Does not Contain a Star |
	// ------------------ + --------------- + ----------------------- +
	// Node is a Leaf     | Impossible      | insert into node        |
	//                    |                 | subdivide               |
	// ------------------ + --------------- + ----------------------- +
	// Node is not a Leaf | insert preexist | insert into the subtree |
	//                    | insert new      |                         |
	// ------------------ + --------------- + ----------------------- +

	// get the node with the given nodeID
	// find out if the node contains a star or not
	containsStar := containsStar(nodeID)

	// find out if the node is a leaf
	isLeaf := isLeaf(nodeID)

	// if the node is a leaf and contains a star
	// subdivide the tree
	// insert the preexisting star into the correct subtree
	// insert the new star into the subtree
	if isLeaf == true && containsStar == true {
		//log.Printf("Case 1, \t %v \t %v", nodeWidth, nodeCenter)
		subdivide(nodeID)
		//tree := printTree(nodeID)

		// Stage 1: Inserting the blocking star
		blockingStarID := getStarID(nodeID)                               // get the id of the star blocking the node
		blockingStar := getStar(blockingStarID)                           // get the actual star
		blockingStarQuadrant := quadrant(blockingStar, nodeID)            // find out in which quadrant it belongs
		quadrantNodeID := getQuadrantNodeID(nodeID, blockingStarQuadrant) // get the nodeID of that quadrant
		insertIntoTree(blockingStarID, quadrantNodeID)                    // insert the star into that node
		removeStarFromNode(nodeID)                                        // remove the blocking star from the node it was blocking

		// Stage 1: Inserting the actual star
		star := getStar(starID)                                  // get the actual star
		starQuadrant := quadrant(star, nodeID)                   // find out in which quadrant it belongs
		quadrantNodeID = getQuadrantNodeID(nodeID, starQuadrant) // get the nodeID of that quadrant
		insertIntoTree(starID, nodeID)
	}

	// if the node is a leaf and does not contain a star
	// insert the star into the node and subdivide it
	if isLeaf == true && containsStar == false {
		//log.Printf("Case 2, \t %v \t %v", nodeWidth, nodeCenter)
		directInsert(starID, nodeID)
	}

	// if the node is not a leaf and contains a star
	// insert the preexisting star into the correct subtree
	// insert the new star into the subtree
	if isLeaf == false && containsStar == true {
		//log.Printf("Case 3, \t %v \t %v", nodeWidth, nodeCenter)
		// Stage 1: Inserting the blocking star
		blockingStarID := getStarID(nodeID)                               // get the id of the star blocking the node
		blockingStar := getStar(blockingStarID)                           // get the actual star
		blockingStarQuadrant := quadrant(blockingStar, nodeID)            // find out in which quadrant it belongs
		quadrantNodeID := getQuadrantNodeID(nodeID, blockingStarQuadrant) // get the nodeID of that quadrant
		insertIntoTree(blockingStarID, quadrantNodeID)                    // insert the star into that node
		removeStarFromNode(nodeID)                                        // remove the blocking star from the node it was blocking

		// Stage 1: Inserting the actual star
		star := getStar(blockingStarID)                          // get the actual star
		starQuadrant := quadrant(star, nodeID)                   // find out in which quadrant it belongs
		quadrantNodeID = getQuadrantNodeID(nodeID, starQuadrant) // get the nodeID of that quadrant
		insertIntoTree(starID, nodeID)
	}

	// if the node is not a leaf and does not contain a star
	// insert the new star into the according subtree
	if isLeaf == false && containsStar == false {
		//log.Printf("Case 4, \t %v \t %v", nodeWidth, nodeCenter)
		star := getStar(starID)                                   // get the actual star
		starQuadrant := quadrant(star, nodeID)                    // find out in which quadrant it belongs
		quadrantNodeID := getQuadrantNodeID(nodeID, starQuadrant) // get the if of that quadrant
		insertIntoTree(starID, quadrantNodeID)                    // insert the star into that quadrant
	}
}

// containsStar returns true if the node with the given id contains a star and returns false if not.
func containsStar(id int64) bool {
	var starID int64

	query := fmt.Sprintf("SELECT star_id FROM nodes WHERE node_id=%d", id)
	err := db.QueryRow(query).Scan(&starID)
	if err != nil {
		log.Fatalf("[ E ] containsStar query: %v\n\t\t\t query: %s\n", err, query)
	}

	if starID != 0 {
		return true
	}

	return false
}

// isLeaf returns true if the node with the given id is a leaf
func isLeaf(nodeID int64) bool {
	var isLeaf bool

	query := fmt.Sprintf("SELECT COALESCE(isleaf, FALSE) FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&isLeaf)
	if err != nil {
		log.Fatalf("[ E ] isLeaf query: %v\n\t\t\t query: %s\n", err, query)
	}

	if isLeaf == true {
		return true
	}

	return false
}

// directInsert inserts the star with the given ID into the given node inside of the given database
func directInsert(starID int64, nodeID int64) {
	// build the query
	query := fmt.Sprintf("UPDATE nodes SET star_id=%d WHERE node_id=%d", starID, nodeID)

	// Execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] directInsert query: %v\n\t\t\t query: %s\n", err, query)
	}
}

// subdivide subdivides the given node creating four child nodes
func subdivide(nodeID int64) {
	boxWidth := getBoxWidth(nodeID)
	boxCenter := getBoxCenter(nodeID)
	originalDepth := getNodeDepth(nodeID)
	timestep := getTimestepNode(nodeID)
	log.Printf("Subdividing %d, setting the timestep to %d", nodeID, timestep)

	// calculate the new positions
	newPosX := boxCenter[0] + (boxWidth / 2)
	newPosY := boxCenter[1] + (boxWidth / 2)
	newNegX := boxCenter[0] - (boxWidth / 2)
	newNegY := boxCenter[1] - (boxWidth / 2)
	newWidth := boxWidth / 2

	// create new news with those positions
	newNodeIDA := newNode(newPosX, newPosY, newWidth, originalDepth+1, timestep)
	newNodeIDB := newNode(newPosX, newNegY, newWidth, originalDepth+1, timestep)
	newNodeIDC := newNode(newNegX, newPosY, newWidth, originalDepth+1, timestep)
	newNodeIDD := newNode(newNegX, newNegY, newWidth, originalDepth+1, timestep)

	// Update the subtrees of the parent node

	// build the query
	query := fmt.Sprintf("UPDATE nodes SET subnode='{%d, %d, %d, %d}', isleaf=FALSE, timestep=%d WHERE node_id=%d", newNodeIDA, newNodeIDB, newNodeIDC, newNodeIDD, timestep, nodeID)

	// Execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] subdivide query: %v\n\t\t\t query: %s\n", err, query)
	}
}

// getBoxWidth gets the width of the box from the node width the given id
func getBoxWidth(nodeID int64) float64 {
	var boxWidth float64

	query := fmt.Sprintf("SELECT box_width FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&boxWidth)
	if err != nil {
		log.Fatalf("[ E ] getBoxWidth query: %v\n\t\t\t query: %s\n", err, query)
	}

	return boxWidth
}

// getTimestepNode gets the timestep of the current node
func getTimestepNode(nodeID int64) int64 {
	var timestep int64

	query := fmt.Sprintf("SELECT timestep FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&timestep)
	if err != nil {
		log.Fatalf("[ E ] getTimeStep query: %v\n\t\t\t query: %s\n", err, query)
	}

	return timestep
}

// getBoxWidth gets the center of the box from the node width the given id
func getBoxCenter(nodeID int64) []float64 {
	var boxCenterX, boxCenterY []uint8

	query := fmt.Sprintf("SELECT box_center[1], box_center[2] FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&boxCenterX, &boxCenterY)
	if err != nil {
		log.Fatalf("[ E ] getBoxCenter query: %v\n\t\t\t query: %s\n", err, query)
	}

	x, parseErr := strconv.ParseFloat(string(boxCenterX), 64)
	y, parseErr := strconv.ParseFloat(string(boxCenterX), 64)

	if parseErr != nil {
		log.Fatalf("[ E ] parse boxCenter: %v\n\t\t\t query: %s\n", err, query)
		log.Fatalf("[ E ] parse boxCenter: (%f, %f)\n", x, y)
	}

	boxCenterFloat := []float64{x, y}

	return boxCenterFloat
}

// getMaxTimestep gets the maximal timestep from the nodes table
func getMaxTimestep() float64 {
	var maxTimestep float64

	query := fmt.Sprintf("SELECT max(timestep) FROM nodes")
	err := db.QueryRow(query).Scan(&maxTimestep)
	if err != nil {
		log.Fatalf("[ E ] getMaxTimestep query: %v\n\t\t\t query: %s\n", err, query)
	}

	return maxTimestep
}

// newNode Inserts a new node into the database with the given parameters
func newNode(x float64, y float64, width float64, depth int64, timestep int64) int64 {
	// build the query creating a new node
	query := fmt.Sprintf("INSERT INTO nodes (box_center, box_width, depth, isleaf, timestep) VALUES ('{%f, %f}', %f, %d, TRUE, %d) RETURNING node_id", x, y, width, depth, timestep)

	var nodeID int64

	// execute the query
	err := db.QueryRow(query).Scan(&nodeID)
	if err != nil {
		log.Fatalf("[ E ] newNode query: %v\n\t\t\t query: %s\n", err, query)
	}

	return nodeID
}

// getStarID returns the id of the star inside of the node with the given ID
func getStarID(nodeID int64) int64 {
	// get the star id from the node
	var starID int64
	query := fmt.Sprintf("SELECT star_id FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&starID)
	if err != nil {
		log.Fatalf("[ E ] getStarID id query: %v\n\t\t\t query: %s\n", err, query)
	}

	return starID
}

// deleteAll Stars deletes all the rows in the stars table
func DeleteAllStars(database *sql.DB) {
	db = database
	// build the query creating a new node
	query := "DELETE FROM stars WHERE TRUE"

	// execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] deleteAllStars query: %v\n\t\t\t query: %s\n", err, query)
	}
}

// deleteAll Stars deletes all the rows in the nodes table
func DeleteAllNodes(database *sql.DB) {
	db = database
	// build the query creating a new node
	query := "DELETE FROM nodes WHERE TRUE"

	// execute the query
	_, err := db.Query(query)
	if err != nil {
		log.Fatalf("[ E ] deleteAllStars query: %v\n\t\t\t query: %s\n", err, query)
	}
}

// getNodeDepth returns the depth of the given node in the tree
func getNodeDepth(nodeID int64) int64 {
	// build the query
	query := fmt.Sprintf("SELECT depth FROM nodes WHERE node_id=%d", nodeID)

	var depth int64

	// Execute the query
	err := db.QueryRow(query).Scan(&depth)
	if err != nil {
		log.Fatalf("[ E ] getNodeDepth query: %v \n\t\t\t query: %s\n", err, query)
	}

	return depth
}

// quadrant returns the quadrant into which the given star belongs
func quadrant(star structs.Star2D, nodeID int64) int64 {
	// get the center of the node the star is in
	center := getBoxCenter(nodeID)
	centerX := center[0]
	centerY := center[1]

	if star.C.X > centerX {
		if star.C.Y > centerY {
			// North East condition
			return 1
		}
		// South East condition
		return 3
	}

	if star.C.Y > centerY {
		// North West condition
		return 0
	}
	// South West condition
	return 2
}

// getQuadrantNodeID returns the id of the requested child-node
// Example: if a parent has four children and quadrant 0 is requested, the function returns the north east child id
func getQuadrantNodeID(parentNodeID int64, quadrant int64) int64 {
	var a, b, c, d []uint8

	// get the star from the stars table
	query := fmt.Sprintf("SELECT subnode[1], subnode[2], subnode[3], subnode[4] FROM nodes WHERE node_id=%d", parentNodeID)
	err := db.QueryRow(query).Scan(&a, &b, &c, &d)
	if err != nil {
		log.Fatalf("[ E ] getQuadrantNodeID star query: %v \n\t\t\tquery: %s\n", err, query)
	}

	returnA, _ := strconv.ParseInt(string(a), 10, 64)
	returnB, _ := strconv.ParseInt(string(b), 10, 64)
	returnC, _ := strconv.ParseInt(string(c), 10, 64)
	returnD, _ := strconv.ParseInt(string(d), 10, 64)

	switch quadrant {
	case 0:
		return returnA
	case 1:
		return returnB
	case 2:
		return returnC
	case 3:
		return returnD
	}

	return -1
}

// getStar returns the star with the given ID from the stars table
func getStar(starID int64) structs.Star2D {
	var x, y, vx, vy, m float64

	// get the star from the stars table
	query := fmt.Sprintf("SELECT x, y, vx, vy, m FROM stars WHERE star_id=%d", starID)
	err := db.QueryRow(query).Scan(&x, &y, &vx, &vy, &m)
	if err != nil {
		log.Fatalf("[ E ] getStar query: %v \n\t\t\tquery: %s\n", err, query)
	}

	star := structs.Star2D{
		C: structs.Vec2{
			X: x,
			Y: y,
		},
		V: structs.Vec2{
			X: vx,
			Y: vy,
		},
		M: m,
	}

	return star
}

// getStarMass returns the mass if the star with the given ID
func getStarMass(starID int64) float64 {
	var mass float64

	// get the star from the stars table
	query := fmt.Sprintf("SELECT m FROM stars WHERE star_id=%d", starID)
	err := db.QueryRow(query).Scan(&mass)
	if err != nil {
		log.Fatalf("[ E ] getStarMass query: %v \n\t\t\tquery: %s\n", err, query)
	}

	return mass
}

// getNodeTotalMass returns the total mass of the node with the given ID and its children
func getNodeTotalMass(nodeID int64) float64 {
	var mass float64

	// get the star from the stars table
	query := fmt.Sprintf("SELECT total_mass FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&mass)
	if err != nil {
		log.Fatalf("[ E ] getStarMass query: %v \n\t\t\tquery: %s\n", err, query)
	}

	return mass
}

// removeStarFromNode removes the star from the node with the given ID
func removeStarFromNode(nodeID int64) {
	// build the query
	query := fmt.Sprintf("UPDATE nodes SET star_id=0 WHERE node_id=%d", nodeID)

	// Execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] removeStarFromNode query: %v\n\t\t\t query: %s\n", err, query)
	}
}

// getListOfStarsGo returns the list of stars in go struct format
func GetListOfStarsGo(database *sql.DB) []structs.Star2D {
	db = database
	// build the query
	query := fmt.Sprintf("SELECT * FROM stars")

	// Execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] removeStarFromNode query: %v\n\t\t\t query: %s\n", err, query)
	}

	var starList []structs.Star2D

	// iterate over the returned rows
	for rows.Next() {

		var starID int64
		var x, y, vx, vy, m float64
		scanErr := rows.Scan(&starID, &x, &y, &vx, &vy, &m)
		if scanErr != nil {
			log.Fatalf("[ E ] scan error: %v", scanErr)
		}

		star := structs.Star2D{
			C: structs.Vec2{
				X: x,
				Y: y,
			},
			V: structs.Vec2{
				X: vx,
				Y: vy,
			},
			M: m,
		}

		starList = append(starList, star)
	}

	return starList
}

// getListOfStarsCsv returns an array of strings containing the coordinates of all the stars in the stars table
func GetListOfStarsCsv(database *sql.DB) []string {
	db = database
	// build the query
	query := fmt.Sprintf("SELECT * FROM stars")

	// Execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] removeStarFromNode query: %v\n\t\t\t query: %s\n", err, query)
	}

	var starList []string

	// iterate over the returned rows
	for rows.Next() {

		var starID int64
		var x, y, vx, vy, m float64
		scanErr := rows.Scan(&starID, &x, &y, &vx, &vy, &m)
		if scanErr != nil {
			log.Fatalf("[ E ] scan error: %v", scanErr)
		}

		row := fmt.Sprintf("%d, %f, %f, %f, %f, %f", starID, x, y, vx, vy, m)
		starList = append(starList, row)
	}

	return starList
}

// getListOfStarsTreeCsv returns an array of strings containing the coordinates of all the stars in the given tree
func GetListOfStarsTree(database *sql.DB, treeindex int64) []structs.Star2D {
	db = database

	// build the query
	query := fmt.Sprintf("SELECT * FROM stars WHERE star_id IN(SELECT star_id FROM nodes WHERE timestep=%d)", treeindex)

	// Execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] removeStarFromNode query: %v\n\t\t\t query: %s\n", err, query)
	}

	var starList []structs.Star2D

	// iterate over the returned rows
	for rows.Next() {

		var starID int64
		var x, y, vx, vy, m float64
		scanErr := rows.Scan(&starID, &x, &y, &vx, &vy, &m)
		if scanErr != nil {
			log.Fatalf("[ E ] scan error: %v", scanErr)
		}

		star := structs.Star2D{
			C: structs.Vec2{
				X: x,
				Y: y,
			},
			V: structs.Vec2{
				X: vx,
				Y: vy,
			},
			M: m,
		}

		starList = append(starList, star)
	}

	return starList
}

// insertList inserts all the stars in the given .csv into the stars and nodes table
func InsertList(database *sql.DB, filename string) {
	db = database
	// open the file
	content, readErr := ioutil.ReadFile(filename)
	if readErr != nil {
		panic(readErr)
	}

	in := string(content)
	reader := csv.NewReader(strings.NewReader(in))

	// insert all the stars into the db
	for {
		record, err := reader.Read()
		if err == io.EOF {
			log.Println("EOF")
			break
		}
		if err != nil {
			log.Println("insertListErr")
			panic(err)
		}

		x, _ := strconv.ParseFloat(record[0], 64)
		y, _ := strconv.ParseFloat(record[1], 64)

		star := structs.Star2D{
			C: structs.Vec2{
				X: x / 100000,
				Y: y / 100000,
			},
			V: structs.Vec2{
				X: 0,
				Y: 0,
			},
			M: 1000,
		}

		fmt.Printf("Inserting (%f, %f)\n", star.C.X, star.C.Y)
		InsertStar(db, star, 1)
	}
}

// getRootNodeID gets a tree index and returns the nodeID of its root node
func getRootNodeID(index int64) int64 {
	var nodeID int64

	query := fmt.Sprintf("SELECT node_id FROM nodes WHERE root_id=%d", index)
	err := db.QueryRow(query).Scan(&nodeID)
	if err != nil {
		log.Fatalf("[ E ] getRootNodeID query: %v\n\t\t\t query: %s\n", err, query)
	}

	return nodeID
}

// updateTotalMass gets a tree index and returns the nodeID of the trees root node
func UpdateTotalMass(database *sql.DB, index int64) {
	db = database
	rootNodeID := getRootNodeID(index)
	log.Printf("RootID: %d", rootNodeID)
	updateTotalMassNode(rootNodeID)
}

// updateTotalMassNode updates the total mass of the given node
func updateTotalMassNode(nodeID int64) float64 {
	var totalmass float64

	// get the subnode ids
	var subnode [4]int64

	query := fmt.Sprintf("SELECT subnode[1], subnode[2], subnode[3], subnode[4] FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&subnode[0], &subnode[1], &subnode[2], &subnode[3])
	if err != nil {
		log.Fatalf("[ E ] updateTotalMassNode query: %v\n\t\t\t query: %s\n", err, query)
	}
	// TODO: implement the getSubtreeIDs(nodeID) []int64 {...} function
	// iterate over all subnodes updating their total masses
	for _, subnodeID := range subnode {
		fmt.Println("----------------------------")
		fmt.Printf("SubdnodeID: %d\n", subnodeID)
		if subnodeID != 0 {
			totalmass += updateTotalMassNode(subnodeID)
		} else {
			// get the starID for getting the star mass
			starID := getStarID(nodeID)
			fmt.Printf("StarID: %d\n", starID)
			if starID != 0 {
				mass := getStarMass(starID)
				log.Printf("starID=%d \t mass: %f", starID, mass)
				totalmass += mass
			}

			// break, this stops a star from being counted multiple (4) times
			break
		}
		fmt.Println("----------------------------")
	}

	query = fmt.Sprintf("UPDATE nodes SET total_mass=%f WHERE node_id=%d", totalmass, nodeID)
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] insert total_mass query: %v\n\t\t\t query: %s\n", err, query)
	}

	fmt.Printf("nodeID: %d \t totalMass: %f\n", nodeID, totalmass)

	return totalmass
}

// updateCenterOfMass recursively updates the center of mass of all the nodes starting at the node with the given
// root index
func UpdateCenterOfMass(database *sql.DB, index int64) {
	db = database
	rootNodeID := getRootNodeID(index)
	log.Printf("RootID: %d", rootNodeID)
	updateCenterOfMassNode(rootNodeID)
}

// updateCenterOfMassNode updates the center of mass of the node with the given nodeID recursively
// center of mass := ((x_1 * m) + (x_2 * m) + ... + (x_n * m)) / m
func updateCenterOfMassNode(nodeID int64) structs.Vec2 {
	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++")

	var centerOfMass structs.Vec2

	// get the subnode ids
	var subnode [4]int64
	var starID int64

	query := fmt.Sprintf("SELECT subnode[1], subnode[2], subnode[3], subnode[4], star_id FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&subnode[0], &subnode[1], &subnode[2], &subnode[3], &starID)
	if err != nil {
		log.Fatalf("[ E ] updateCenterOfMassNode query: %v\n\t\t\t query: %s\n", err, query)
	}

	// if the nodes does not contain a star but has children, update the center of mass
	if subnode != ([4]int64{0, 0, 0, 0}) {
		log.Println("[   ] recursing deeper")

		// define variables storing the values of the subnodes
		var totalMass float64
		var centerOfMassX float64
		var centerOfMassY float64

		// iterate over all the subnodes and calculate the center of mass of each node
		for _, subnodeID := range subnode {
			subnodeCenterOfMass := updateCenterOfMassNode(subnodeID)

			if subnodeCenterOfMass.X != 0 && subnodeCenterOfMass.Y != 0 {
				fmt.Printf("SubnodeCenterOfMass: (%f, %f)\n", subnodeCenterOfMass.X, subnodeCenterOfMass.Y)
				subnodeMass := getNodeTotalMass(subnodeID)
				totalMass += subnodeMass

				centerOfMassX += subnodeCenterOfMass.X * subnodeMass
				centerOfMassY += subnodeCenterOfMass.Y * subnodeMass
			}
		}

		// calculate the overall center of mass of the subtree
		centerOfMass = structs.Vec2{
			X: centerOfMassX / totalMass,
			Y: centerOfMassY / totalMass,
		}

		// else, use the star as the center of mass (this can be done, because of the rule defining that there
		// can only be one star in a cell)
	} else {
		log.Println("[   ] using the star in the node as the center of mass")
		log.Printf("[   ] NodeID: %v", nodeID)
		starID := getStarID(nodeID)

		if starID == 0 {
			log.Println("[   ] StarID == 0...")
			centerOfMass = structs.Vec2{
				X: 0,
				Y: 0,
			}
		} else {
			log.Printf("[   ] NodeID: %v", starID)
			star := getStar(starID)
			centerOfMassX := star.C.X
			centerOfMassY := star.C.Y
			centerOfMass = structs.Vec2{
				X: centerOfMassX,
				Y: centerOfMassY,
			}
		}
	}

	// build the query
	query = fmt.Sprintf("UPDATE nodes SET center_of_mass='{%f, %f}' WHERE node_id=%d", centerOfMass.X, centerOfMass.Y, nodeID)

	// Execute the query
	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		log.Fatalf("[ E ] update center of mass query: %v\n\t\t\t query: %s\n", err, query)
	}

	fmt.Printf("[   ] CenterOfMass: (%f, %f)\n", centerOfMass.X, centerOfMass.Y)

	return centerOfMass
}

// genForestTree generates a forest representation of the tree with the given index
func GenForestTree(database *sql.DB, index int64) string {
	db = database
	rootNodeID := getRootNodeID(index)
	return genForestTreeNode(rootNodeID)
}

// genForestTreeNodes returns a sub-representation of a given node in forest format
func genForestTreeNode(nodeID int64) string {
	var returnString string

	// get the subnode ids
	var subnode [4]int64

	query := fmt.Sprintf("SELECT subnode[1], subnode[2], subnode[3], subnode[4] FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&subnode[0], &subnode[1], &subnode[2], &subnode[3])
	if err != nil {
		log.Fatalf("[ E ] updateTotalMassNode query: %v\n\t\t\t query: %s\n", err, query)
	}

	returnString += "["

	// iterate over all subnodes updating their total masses
	for _, subnodeID := range subnode {
		if subnodeID != 0 {
			centerOfMass := getCenterOfMass(nodeID)
			mass := getNodeTotalMass(nodeID)
			returnString += fmt.Sprintf("%.0f %.0f %.0f", centerOfMass.X, centerOfMass.Y, mass)
			returnString += genForestTreeNode(subnodeID)
		} else {
			if getStarID(nodeID) != 0 {
				coords := getStarCoordinates(nodeID)
				starID := getStarID(nodeID)
				mass := getStarMass(starID)
				returnString += fmt.Sprintf("[%.0f %.0f %.0f]", coords.X, coords.Y, mass)
			} else {
				returnString += fmt.Sprintf("[0 0]")
			}
			// break, this stops a star from being counted multiple (4) times
			break
		}
	}

	returnString += "]"

	return returnString
}

// getCenterOfMass returns the center of mass of the given nodeID
func getCenterOfMass(nodeID int64) structs.Vec2 {

	var CenterOfMass [2]float64

	// get the star from the stars table
	query := fmt.Sprintf("SELECT center_of_mass[1], center_of_mass[2] FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&CenterOfMass[0], &CenterOfMass[1])
	if err != nil {
		log.Fatalf("[ E ] getCenterOfMass query: %v \n\t\t\tquery: %s\n", err, query)
	}

	return structs.Vec2{X: CenterOfMass[0], Y: CenterOfMass[1]}
}

// getStarCoordinates gets the star coordinates of a star using a given nodeID.
// It returns a vector describing the coordinates
func getStarCoordinates(nodeID int64) structs.Vec2 {
	var Coordinates [2]float64

	starID := getStarID(nodeID)

	// get the star from the stars table
	query := fmt.Sprintf("SELECT x, y FROM stars WHERE star_id=%d", starID)
	err := db.QueryRow(query).Scan(&Coordinates[0], &Coordinates[1])
	if err != nil {
		log.Fatalf("[ E ] getStarCoordinates query: %v \n\t\t\tquery: %s\n", err, query)
	}

	fmt.Printf("%v\n", Coordinates)

	return structs.Vec2{X: Coordinates[0], Y: Coordinates[1]}
}

// CalcAllForces calculates all the forces acting on the given star.
// The theta value it receives is used by the Barnes-Hut algorithm to determine what
// stars to include into the calculations
func CalcAllForces(database *sql.DB, star structs.Star2D, galaxyIndex int64, theta float64) structs.Vec2 {
	db = database

	// calculate all the forces and add them to the list of all forces
	// this is done recursively
	// first of all, get the root id
	log.Println("[db_actions] Getting the root ID")
	rootID := getRootNodeID(galaxyIndex)
	log.Println("[db_actions] Done getting the root ID")

	log.Printf("[db_actions] Calculating the forces acting on the star %v", star)
	force := CalcAllForcesNode(star, rootID, theta)
	log.Printf("[db_actions] Done calculating the forces acting on the star %v", star)
	log.Printf("[db_actions] Force: %v", force)

	return force
}

// calcAllForces nodes calculates the forces in between a sta	log.Printf("Calculating the forces acting on the star %v", star)r and a node and returns the overall force
// TODO: implement the calcForce(star, centerOfMass) {...} function
// TODO: implement the getSubtreeIDs(nodeID) []int64 {...} function
func CalcAllForcesNode(star structs.Star2D, nodeID int64, theta float64) structs.Vec2 {
	log.Println("---------------------------------------")
	log.Printf("NodeID: %d \t star: %v \t theta: %f \t nodeboxwidth: %f", nodeID, star, theta, getBoxWidth(nodeID))
	var forceX float64
	var forceY float64
	var localTheta float64

	if nodeID != 0 {
		log.Println("Calculating the localtheta")
		localTheta = calcTheta(star, nodeID)
		log.Printf("Done calculating theta: %v", localTheta)
	}

	if localTheta < theta {
		log.Printf("localTheta < theta")
		var force structs.Vec2

		// if the nodeID is not zero, use the center of mass as the other star
		if nodeID != 0 {
			pseudoStarCoodinates := getCenterOfMass(nodeID)
			PseudoStar := structs.Star2D{
				C: structs.Vec2{
					X: pseudoStarCoodinates.X,
					Y: pseudoStarCoodinates.Y,
				},
				V: structs.Vec2{
					X: 0,
					Y: 0,
				},
				M: 1000,
			}
			log.Printf("PseudoStar: %v", PseudoStar)
			force = calcForce(star, PseudoStar)

			// else, use the star in the node as the other star
		} else {
			if getStarID(nodeID) != 0 {
				var pseudoStar = getStar(getStarID(nodeID))
				force = calcForce(star, pseudoStar)
			}
		}

		forceX = force.X
		forceY = force.X
	} else {
		log.Printf("localTheta > theta")
		// iterate over all subtrees and add the forces acting through them
		var subtreeIDs [4]int64
		subtreeIDs = getSubtreeIDs(nodeID)
		for _, subtreeID := range subtreeIDs {

			// don't recurse into
			if subtreeID != 0 {
				var force = CalcAllForcesNode(star, subtreeID, theta)
				log.Printf("force: %v", force)
				forceX += force.X
				forceY += force.Y
			}
		}
	}
	return structs.Vec2{forceX, forceY}
}

// calcTheta calculates the theat for a given star and a node
func calcTheta(star structs.Star2D, nodeID int64) float64 {
	d := getBoxWidth(nodeID)
	r := distance(star, nodeID)
	theta := d / r
	return theta
}

// calculate the distance in between the star and the node with the given ID
func distance(star structs.Star2D, nodeID int64) float64 {
	var starX float64 = star.C.X
	var starY float64 = star.C.Y
	var node structs.Vec2 = getNodeCenterOfMass(nodeID)
	var nodeX float64 = node.X
	var nodeY float64 = node.Y

	var tmpX = math.Pow(starX-nodeX, 2)
	var tmpY = math.Pow(starY-nodeY, 2)

	var distance float64 = math.Sqrt(tmpX + tmpY)
	return distance
}

// getNodeCenterOfMass returns the center of mass of the node with the given ID
func getNodeCenterOfMass(nodeID int64) structs.Vec2 {
	var Coordinates [2]float64

	// get the star from the stars table
	query := fmt.Sprintf("SELECT center_of_mass[1], center_of_mass[2] FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&Coordinates[0], &Coordinates[1])
	if err != nil {
		log.Fatalf("[ E ] getNodeCenterOfMass query: %v \n\t\t\tquery: %s\n", err, query)
	}

	return structs.Vec2{X: Coordinates[0], Y: Coordinates[1]}
}

// getSubtreeIDs returns the id of the subtrees of the nodeID
func getSubtreeIDs(nodeID int64) [4]int64 {

	var subtreeIDs [4]int64

	// get the star from the stars table
	query := fmt.Sprintf("SELECT subnode[1], subnode[2], subnode[3], subnode[4] FROM nodes WHERE node_id=%d", nodeID)
	err := db.QueryRow(query).Scan(&subtreeIDs[0], &subtreeIDs[1], &subtreeIDs[2], &subtreeIDs[3])
	if err != nil {
		log.Fatalf("[ E ] getSubtreeIDs query: %v \n\t\t\tquery: %s\n", err, query)
	}

	return subtreeIDs
}

func calcForce(s1 structs.Star2D, s2 structs.Star2D) structs.Vec2 {
	log.Println("+++++++++++++++++++++++++")
	log.Printf("Calculating the force acting inbetween %f and %f", s1, s2)
	G := 6.6726 * math.Pow(10, -11)

	// calculate the force acting
	var combinedMass float64 = s1.M * s2.M
	var distance float64 = math.Sqrt(math.Pow(math.Abs(s1.C.X-s2.C.X), 2) + math.Pow(math.Abs(s1.C.Y-s2.C.Y), 2))

	var scalar float64 = G * ((combinedMass) / math.Pow(distance, 2))

	// define a unit vector pointing from s1 to s2
	var vector structs.Vec2 = structs.Vec2{s2.C.X - s1.C.X, s2.C.Y - s1.C.Y}
	var UnitVector structs.Vec2 = structs.Vec2{vector.X / distance, vector.Y / distance}

	// multiply the vector with the force to get a vector representing the force acting
	var force structs.Vec2 = UnitVector.Multiply(scalar)
	log.Println("+++++++++++++++++++++++++")

	// return the force exerted on s1 by s2
	return force
}
