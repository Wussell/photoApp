package main

/*
type photoInfo struct {
	photoID   int
	albumID   int
	userID    int
	tagged_id int
	time	time.Time
}
*/

import (
	"database/sql"
	"fmt"
	"html/template"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

const dbInit = "CREATE TABLE users (id integer primary key, email text unique);\n" +
	"CREATE TABLE albums (id integer primary key, user_id integer references users(id), name text not null);\n" +
	"CREATE TABLE photos (id integer primary key, album_id integer references albums(id), user_id integer references users(id), path TEXT UNIQUE);\n" +
	"CREATE TABLE album_permissions (album_id INTEGER REFERENCES albums(id), user_id INTEGER REFERENCES users(id));\n" +
	"INSERT INTO users (email) VALUES ('user1@example.com');\n" +
	"INSERT INTO users (email) VALUES ('user2@example.com');\n" +
	"INSERT INTO albums (user_id, name) VALUES (1, '1 main');\n" +
	"INSERT INTO albums (user_id, name) VALUES (2, '2 main');\n" +
	"INSERT INTO albums (user_id, name) VALUES (1, '1s Birthday!');\n" +
	"INSERT INTO album_permissions (album_id, user_id) VALUES (1,1);\n" +
	"INSERT INTO album_permissions (album_id, user_id) VALUES (1,2);\n" +
	"INSERT INTO album_permissions (album_id, user_id) VALUES (2,2);\n" +
	"INSERT INTO photos (album_id, user_id, path) VALUES (1, 1, '/Users/moose1/Documents/photoApp/Photos/1.jpg');\n" +
	"INSERT INTO photos (album_id, user_id, path) VALUES (1, 1, '/Users/moose1/Documents/photoApp/Photos/2.png');\n"

// these functions are to be used with a database that includes following tables (! = primary key):
// users: id!|email		albums: id!|userid|name	     photos: id!|album_id|user_id|path		album_permissions: album_id|user_id	tags: photo_id|tagged_id

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

// create a new user along with an initial album
func newUser(email string, db *sql.DB) {
	r, err := db.Exec("insert into users (email) values (?)", email)
	check(err)

	userID, err := r.LastInsertId()
	check(err)
	mainAlbum := fmt.Sprintf("%s's Photos", email)
	r, err = db.Exec("insert into albums (user_id, name) values (?, ?)", userID, mainAlbum)
	check(err)
	albumID, err := r.LastInsertId()
	check(err)
	givePerm(albumID, userID, db)
}

func newAlbum(name string, userID int64, db *sql.DB) {
	r, err := db.Exec("insert into albums (name, user_id) values (?, ?)", name, userID)
	check(err)
	albumID, err := r.LastInsertId()
	check(err)
	givePerm(albumID, userID, db)
}

// checks if the given user has permission to access the given album
func checkPerm(albumID int64, userID int64, db *sql.DB) bool {
	//retrieve all albums that a user has access to
	permittedAlbumRows, err := db.Query("select album_id from album_permissions where user_id = ? and album_id = ?", userID, albumID)
	defer permittedAlbumRows.Close()
	check(err)
	// copy all album ids that the specified user has access to into a slice
	permittedAlbum := make([]int64, 0)
	for i := 0; permittedAlbumRows.Next(); i++ {
		var newElem int64
		permittedAlbum = append(permittedAlbum, newElem)
		err = permittedAlbumRows.Scan(&permittedAlbum[i])
		check(err)
		//i++
	}
	var hasPerm bool
	if len(permittedAlbum) > 0 {
		hasPerm = true
	}
	return hasPerm
}

// add a photo to a specified album if the calling user has permission according to the album_permissions table
func addPhoto(albumID int64, userID int64, db *sql.DB) (int64, string) {
	var photoID int64
	var path string
	if checkPerm(albumID, userID, db) == true {
		res, err := db.Exec("INSERT INTO photos (user_id, album_id) VALUES (?, ?)", userID, albumID)
		check(err)

		photoID, err = res.LastInsertId()
		check(err)
		path = "/Users/moose1/Documents/photoApp/Photos/" + strconv.FormatInt(photoID, 10) //TODO: get image format
		_, err = db.Exec("UPDATE photos SET path = ? WHERE id = ?", path, photoID)
		check(err)
	} else {
		fmt.Printf("That user doesn't have permission to access the album!\n")
	}
	return photoID, path
	//add a tag feature to this function?
}

// give a user permission to view and add photos to an album
func givePerm(albumID int64, userID int64, db *sql.DB) {
	if checkPerm(albumID, userID, db) == false {
		_, err := db.Exec("insert into album_permissions (album_id, user_id) values (?, ?)", albumID, userID)
		check(err)
	} else {
		fmt.Printf("That user already has permission to access the album!\n")
	}
}

func showTags(userID int64, db *sql.DB) ([]int64, []int64) {
	taggedPhotoRows, err := db.Query("SELECT id FROM photos JOIN tags ON photos.id = tags.photo_id WHERE tagged_user_id = ?", userID)
	defer taggedPhotoRows.Close()
	check(err)

	taggedPhotos := make([]int64, 0)
	for i := 0; taggedPhotoRows.Next(); i++ {
		var newElem int64
		taggedPhotos = append(taggedPhotos, newElem)
		err := taggedPhotoRows.Scan(&taggedPhotos[i])
		check(err)
	}

	taggedAlbumRows, err := db.Query("SELECT album_id FROM photos JOIN tags ON photos.id = tags.photo_id WHERE tagged_user_id = ?", userID)
	defer taggedAlbumRows.Close()
	check(err)

	taggedAlbums := make([]int64, 0)
	for i := 0; taggedAlbumRows.Next(); i++ {
		var newElem int64
		taggedAlbums = append(taggedAlbums, newElem)
		err := taggedAlbumRows.Scan(&taggedAlbums[i])
		check(err)
	}
	return taggedPhotos, taggedAlbums
}

var templates = template.Must(template.ParseFiles("templates/home.html", "templates/album.html", "templates/photo.html"))

type page interface {
	query() string
	render(w http.ResponseWriter, r *http.Request, rows *sql.Rows)
}

type homepage struct {
	UserID int64
	Albums []int64
}

type albumpage struct {
	AlbumID int64
	Photos  []int64
}

type photopage struct {
	AlbumID int64
	PhotoID int64
	Path    string
}

func (h homepage) query() string {
	//TODO: get user_id from http request header
	return "SELECT id FROM albums WHERE user_id = " + strconv.FormatInt(h.UserID, 10)
}

func (a albumpage) query() string {
	return "SELECT id FROM photos WHERE album_id = " + strconv.FormatInt(a.AlbumID, 10)
}

func (p photopage) render(w http.ResponseWriter) error {
	return templates.ExecuteTemplate(w, "photo.html", p)
}

// TODO: handle errors instead of panicking
func (h homepage) render(w http.ResponseWriter, r *http.Request, rows *sql.Rows) error {
	albums := make([]int64, 0)
	for i := 0; rows.Next(); i++ {
		var newElem int64
		albums = append(albums, newElem)
		err := rows.Scan(&albums[i])
		if err != nil {
			return err
		}
	}
	h.Albums = albums

	return templates.ExecuteTemplate(w, "home.html", h)
}

func (a albumpage) render(w http.ResponseWriter, r *http.Request, rows *sql.Rows) error {
	photos := make([]int64, 0)
	for i := 0; rows.Next(); i++ {
		var newElem int64
		photos = append(photos, newElem)
		err := rows.Scan(&photos[i])
		if err != nil {
			return err
		}
	}
	a.Photos = photos
	fmt.Printf("album photos: %v\n", a.Photos)
	return templates.ExecuteTemplate(w, "album.html", a)
}

func homeHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	h := homepage{}
	m := validPath.FindStringSubmatch(r.URL.Path)
	id, err := strconv.ParseInt(m[2], 10, 64)
	h.UserID = id
	check(err)
	rows, err := db.Query(h.query())
	defer rows.Close()
	if err != nil {
		log.Panic("ERROR: invalid user id\n")
	}
	err = h.render(w, r, rows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func albumHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	a := albumpage{}
	m := validPath.FindStringSubmatch(r.URL.Path)
	id, err := strconv.ParseInt(m[2], 10, 64)
	a.AlbumID = id
	check(err)
	fmt.Printf("album query: %s \n", a.query())
	rows, err := db.Query(a.query())
	defer rows.Close()
	if err != nil {
		log.Panic("ERROR: invalid album\n")
	}
	err = a.render(w, r, rows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var validPath = regexp.MustCompile("^/(home|album|photo|photos|upload)/([a-zA-Z0-9]+)$")

// serves HTML
func photoHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	p := photopage{}
	m := validPath.FindStringSubmatch(r.URL.Path)

	var err error
	id, err := strconv.ParseInt(m[2], 10, 64)
	check(err)
	p.PhotoID = id

	err = p.render(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// serves images /photos/1 -> /Users/moose1/Documents/photoApp/Photos/1.jpg
func photosHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	fmt.Printf("Photos path request: % +v\n", r.URL.Path)
	m := validPath.FindStringSubmatch(r.URL.Path)
	id, err := strconv.ParseInt(m[2], 10, 64)
	check(err)
	var path string
	err = db.QueryRow("SELECT path FROM photos WHERE id = ?", id).Scan(&path)
	if err != nil {
		log.Fatalf("path query err \n")
	}
	f, err := os.Open(path)
	check(err)
	_, err = io.Copy(w, f)
	check(err)
}

func uploadHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	err := r.ParseMultipartForm(1000000)
	check(err)
	imageInput := r.MultipartForm.File
	fh := imageInput["photo"]
	if len(fh) < 1 {
		log.Fatalf("ERR: no file uploaded")
	} else if len(fh) > 1 {
		log.Fatalf("ERR: too many files uploaded\n")
	}
	fmt.Printf("uploaded file size: %v\n", fh[0].Size)
	mpf, err := fh[0].Open()
	defer mpf.Close()
	check(err)

	m := validPath.FindStringSubmatch(r.URL.Path)
	albumID, err := strconv.ParseInt(m[2], 10, 64)
	check(err)
	//TODO: Get userID from site token or cookie
	photoID, path := addPhoto(albumID, 1, db)
	f, err := os.Create(path)
	check(err)
	_, err = io.Copy(f, mpf)
	check(err)
	info, err := f.Stat()
	check(err)
	fmt.Printf("copied file size: %v\n", info.Size())
	http.Redirect(w, r, "/photo/"+strconv.FormatInt(photoID, 10), http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, *sql.DB), db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, db)
	}
}

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	check(err)
	defer db.Close()

	_, err = db.Exec(dbInit)
	check(err)
	log.SetFlags(log.Lshortfile)
	http.HandleFunc("/home/", makeHandler(homeHandler, db))
	http.HandleFunc("/album/", makeHandler(albumHandler, db))
	http.HandleFunc("/photo/", makeHandler(photoHandler, db))
	http.HandleFunc("/photos/", makeHandler(photosHandler, db))
	http.HandleFunc("/upload/", makeHandler(uploadHandler, db)) //TODO: change upload path and regexp parser

	log.Fatal(http.ListenAndServe(":8080", nil))
}
