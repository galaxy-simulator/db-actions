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
		database *sql.DB
		star     structs.Star2D
		theta    float64
	}
	tests := []struct {
		name string
		args args
		want structs.Vec2
	}{
		{
			name: "force acting on a single star",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalcAllForces(tt.args.database, tt.args.star, tt.args.theta); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CalcAllForces() = %v, want %v", got, tt.want)
			}
		})
	}
}
