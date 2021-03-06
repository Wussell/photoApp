package main

import (
	"database/sql"
	"os"
	"strconv"
	"testing"
)

const dbInit = "CREATE TABLE users (id integer primary key, email text unique);\n" +
	"CREATE TABLE albums (id integer primary key, user_id integer references users(id), name text not null);\n" +
	"CREATE TABLE photos (id integer primary key, album_id integer references albums(id), user_id integer references users(id));\n" +
	"CREATE TABLE album_permissions (album_id integer references albums(id), user_id integer references users(id), unique (album_id, user_id));\n" +
	"CREATE TABLE tags (photo_id references photos(id), tagged_user_id references users(id), unique (photo_id, tagged_user_id));\n" +
	"INSERT INTO users (email) VALUES ('user1@example.com');\n" +
	"INSERT INTO users (email) VALUES ('user2@example.com');\n" +
	"INSERT INTO users (email) VALUES ('user3@example.com');\n" +
	"INSERT INTO albums (user_id, name) VALUES (1, '1 main');\n" +
	"INSERT INTO albums (user_id, name) VALUES (2, '2 main');\n" +
	"INSERT INTO albums (user_id, name) VALUES (1, '1s Birthday!');\n" +
	"INSERT INTO albums (user_id, name) VALUES (3, '3 main');\n" +
	"INSERT INTO photos (album_id, user_id) VALUES (1, 1);\n" +
	"INSERT INTO photos (album_id, user_id) VALUES (2, 2);\n" +
	"INSERT INTO photos (album_id, user_id) VALUES (3, 1);\n" +
	"INSERT INTO photos (album_id, user_id) VALUES (4, 3);\n"

func TestPerm(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	check(err)
	defer db.Close()

	_, err = db.Exec(dbInit + "INSERT INTO album_permissions (album_id, user_id) VALUES (1, 1);\n" +
		"INSERT INTO album_permissions (album_id, user_id) VALUES (2, 2);\n" +
		"INSERT INTO album_permissions (album_id, user_id) VALUES (3, 2);\n")
	check(err)

	examples := []struct {
		name    string
		albumID int64
		userID  int64
		want    bool
	}{
		{
			name:    "perm",
			albumID: 1,
			userID:  1,
			want:    true,
		},
		{
			name:    "noPerm",
			albumID: 1,
			userID:  2,
			want:    false,
		},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			got := checkPerm(ex.albumID, ex.userID, db)
			if got != ex.want {
				t.Fatalf("got %v, want %v\n", got, ex.want)
			}
		})
	}
}

func TestTags(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	check(err)
	defer db.Close()

	_, err = db.Exec(dbInit + "INSERT INTO tags (photo_id, tagged_user_id) VALUES (3, 2);\n" +
		"INSERT INTO tags (photo_id, tagged_user_id) VALUES (4, 1);\n" +
		"INSERT INTO tags (photo_id, tagged_user_id) VALUES (4, 2);\n")
	check(err)

	examples := []struct {
		name       string
		userID     int64
		wantPhotos []int64
		wantAlbums []int64
	}{
		{
			name:       "no tags",
			userID:     3,
			wantPhotos: []int64{},
			wantAlbums: []int64{},
		},
		{
			name:       "one tag",
			userID:     1,
			wantPhotos: []int64{4},
			wantAlbums: []int64{4},
		},
		{
			name:       "two tags",
			userID:     2,
			wantPhotos: []int64{3, 4},
			wantAlbums: []int64{3, 4},
		},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			gotPhotos, gotAlbums := showTags(ex.userID, db)
			for i := range gotPhotos {
				if gotPhotos[i] != ex.wantPhotos[i] {
					t.Fatalf("got photo %v, want photo %v\n", gotPhotos[i], ex.wantPhotos[i])
				}
			}
			for i := range gotAlbums {
				if gotAlbums[i] != ex.wantAlbums[i] {
					t.Fatalf("got album %v, want album %v\n", gotAlbums[i], ex.wantAlbums[i])
				}
			}
		})
	}
}

func TestAddPhoto(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	check(err)
	defer db.Close()

	_, err = db.Exec(dbInit + "INSERT INTO album_permissions (album_id, user_id) VALUES (1, 1);\n" +
		"INSERT INTO album_permissions (album_id, user_id) VALUES (2, 2);\n" +
		"INSERT INTO album_permissions (album_id, user_id) VALUES (3, 2);\n")
	check(err)

	examples := []struct {
		name      string
		user      int64
		album     int64
		photoPath string
	}{
		{
			name:      "basic add",
			user:      1,
			album:     1,
			photoPath: "/Users/moose1/Documents/reference/Scrampy.jpg",
		},
		{
			name:      "cross user add",
			user:      2,
			album:     3,
			photoPath: "/Users/moose1/Documents/reference/Toph.png",
		},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			photoId := addPhoto(ex.album, ex.user, ex.photoPath, db)

			photoRow := db.QueryRow("SELECT user_id FROM photos WHERE id = ?", photoId)
			var userId int64
			if err = photoRow.Scan(&userId); err != nil {
				t.Fatalf("ERR: %s\n", err)
			}

			err = os.Remove("/Users/moose1/Documents/photoApp/Photos/" + strconv.FormatInt(photoId, 10))
		})
	}
}
