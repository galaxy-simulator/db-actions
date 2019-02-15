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
	"reflect"
	"testing"

	"git.darknebu.la/GalaxySimulator/structs"
	_ "github.com/lib/pq"
)

func TestCalcAllForces(t *testing.T) {
	// define a database
	db = ConnectToDB()
	db.SetMaxOpenConns(75)

	type args struct {
		database    *sql.DB
		star        structs.Star2D
		galaxyIndex int64
		theta       float64
	}
	tests := []struct {
		name string
		args args
		want structs.Vec2
	}{
		{
			name: "star in the top right quadrant",
			args: args{
				database: db,
				star: structs.Star2D{
					C: structs.Vec2{
						X: 100,
						Y: 100,
					},
					V: structs.Vec2{
						X: 0,
						Y: 0,
					},
					M: 1000,
				},
				theta: 0.5,
			},
			want: structs.Vec2{
				X: 0,
				Y: 0,
			},
		},
		{
			name: "star in the bottom left quadrant",
			args: args{
				database: db,
				star: structs.Star2D{
					C: structs.Vec2{
						X: -100,
						Y: -100,
					},
					V: structs.Vec2{
						X: 0,
						Y: 0,
					},
					M: 1000,
				},
				theta: 0.5,
			},
			want: structs.Vec2{
				X: 0,
				Y: 0,
			},
		},
		{
			name: "star in the far top right quadrant",
			args: args{
				database: db,
				star: structs.Star2D{
					C: structs.Vec2{
						X: 490,
						Y: 490,
					},
					V: structs.Vec2{
						X: 0,
						Y: 0,
					},
					M: 1000,
				},
				theta: 0.5,
			},
			want: structs.Vec2{
				X: 0,
				Y: 0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalcAllForces(tt.args.database, tt.args.star, tt.args.galaxyIndex, tt.args.theta); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CalcAllForces() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInsertStar(t *testing.T) {
	// define a database
	db = ConnectToDB()
	db.SetMaxOpenConns(75)

	type args struct {
		database *sql.DB
		star     structs.Star2D
		index    int64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Insert a star into the database",
			args: args{
				database: db,
				star: structs.Star2D{
					C: structs.Vec2{
						X: 100,
						Y: 100,
					},
					V: structs.Vec2{
						X: 0,
						Y: 0,
					},
					M: 1000,
				},
				index: 1,
			},
		},
		{
			name: "Insert a star into the database",
			args: args{
				database: db,
				star: structs.Star2D{
					C: structs.Vec2{
						X: 150,
						Y: 150,
					},
					V: structs.Vec2{
						X: 0,
						Y: 0,
					},
					M: 1000,
				},
				index: 1,
			},
		},
		{
			name: "Insert a star into the database",
			args: args{
				database: db,
				star: structs.Star2D{
					C: structs.Vec2{
						X: 150,
						Y: 150,
					},
					V: structs.Vec2{
						X: 0,
						Y: 0,
					},
					M: 1000,
				},
				index: 2,
			},
		},
		{
			name: "Insert a star into the database",
			args: args{
				database: db,
				star: structs.Star2D{
					C: structs.Vec2{
						X: 100,
						Y: 100,
					},
					V: structs.Vec2{
						X: 0,
						Y: 0,
					},
					M: 1000,
				},
				index: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InsertStar(tt.args.database, tt.args.star, tt.args.index)
		})
	}
}
