package files

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/Nanocloud/community/nanocloud/models/users"
	"github.com/Nanocloud/community/nanocloud/oauth2"
	log "github.com/Sirupsen/logrus"
	"github.com/labstack/echo"
)

type hash map[string]interface{}

type file_t struct {
	Id         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes"`
}

/*
func Post(c *echo.Context) error {
	mr, err := c.Request().MultipartReader()
	if err != nil {
		log.Println(err)
		return err
	}
	for {
		p, err := mr.NextPart()
		if err != nil {
			log.Println(err)
			return err
		}
		if err == io.EOF {
			return err
		}
		if err != nil {
			log.Fatal(err)
		}
		var outfile *os.File
		if outfile, err = os.Create("C:/Users/Administrator/Desktop/" + p.FileName()); nil != err {
			return err
		}
		if _, err = io.Copy(outfile, p); nil != err {
			return err
		}
	}
	return nil
}*/

var kUploadDir string

// Get checks a chunk.
// If it doesn't exist then flowjs tries to upload it via Post.
func GetUpload(w http.ResponseWriter, r *http.Request) {
	user, oauthErr := oauth2.GetUser(w, r)
	if user == nil || oauthErr != nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	kUploadDir = "C:/Users/"
	chunkPath := filepath.Join(
		kUploadDir,
		user.(*users.User).Id,
		"incomplete",
		r.FormValue("flowFilename"),
		r.FormValue("flowChunkNumber"),
	)
	if _, err := os.Stat(chunkPath); err != nil {
		http.Error(w, "chunk not found", http.StatusSeeOther)
		return
	}
}

// Post tries to get and save a chunk.
func Post(w http.ResponseWriter, r *http.Request) {
	kUploadDir = "C:/Users/"

	// get the multipart data
	err := r.ParseMultipartForm(2 * 1024 * 1024) // chunkSize
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	chunkNum, err := strconv.Atoi(r.FormValue("flowChunkNumber"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	totalChunks, err := strconv.Atoi(r.FormValue("flowTotalChunks"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := r.FormValue("flowFilename")
	// module := r.FormValue("module")

	err = writeChunk(filepath.Join(kUploadDir, "incomplete", filename), strconv.Itoa(chunkNum), r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// it's done if it's not the last chunk
	if chunkNum < totalChunks {
		return
	}

	upPath := filepath.Join(kUploadDir, filename)

	// now finish the job
	err = assembleUpload(kUploadDir, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to assemble the uploaded chunks")
		return
	}
	log.WithFields(log.Fields{
		"path": upPath,
	}).Info("file uploaded")

}

func writeChunk(path, chunkNum string, r *http.Request) error {
	// prepare the chunk folder
	err := os.MkdirAll(path, 02750)
	if err != nil {
		return err
	}
	// write the chunk
	fileIn, _, err := r.FormFile("file")
	if err != nil {
		return err
	}
	defer fileIn.Close()
	fileOut, err := os.Create(filepath.Join(path, chunkNum))
	if err != nil {
		return err
	}
	defer fileOut.Close()
	_, err = io.Copy(fileOut, fileIn)
	return err
}

func assembleUpload(path, filename string) error {

	// create final file to write to
	dst, err := os.Create(filepath.Join(path, filename))
	if err != nil {
		return err
	}
	defer dst.Close()

	chunkDirPath := filepath.Join(path, "incomplete", filename)
	fileInfos, err := ioutil.ReadDir(chunkDirPath)
	if err != nil {
		return err
	}
	sort.Sort(byChunk(fileInfos))
	for _, fs := range fileInfos {
		src, err := os.Open(filepath.Join(chunkDirPath, fs.Name()))
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, src)
		src.Close()
		if err != nil {
			return err
		}
	}
	os.RemoveAll(chunkDirPath)

	return nil
}

type byChunk []os.FileInfo

func (a byChunk) Len() int      { return len(a) }
func (a byChunk) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byChunk) Less(i, j int) bool {
	ai, _ := strconv.Atoi(a[i].Name())
	aj, _ := strconv.Atoi(a[j].Name())
	return ai < aj
}

func Get(c *echo.Context) error {
	path := c.Query("path")
	showHidden := c.Query("show_hidden") == "true"
	create := c.Query("create") == "true"

	if len(path) < 1 {
		return c.JSON(
			http.StatusBadRequest,
			hash{
				"error": "Path not specified",
			},
		)
	}

	s, err := os.Stat(path)
	if err != nil {
		fmt.Println(err.(*os.PathError).Err.Error())
		m := err.(*os.PathError).Err.Error()
		if m == "no such file or directory" || m == "The system cannot find the file specified." {
			if create {
				err := os.MkdirAll(path, 0777)
				if err != nil {
					return err
				}
				s, err = os.Stat(path)
				if err != nil {
					return err
				}
			} else {
				return c.JSON(
					http.StatusNotFound,
					hash{
						"error": "no such file or directory",
					},
				)
			}
		} else {
			return err
		}
	}

	if s.Mode().IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		files, err := f.Readdir(-1)
		if err != nil {
			return err
		}

		rt := make([]file_t, 0)

		for _, file := range files {
			name := file.Name()
			if !showHidden && name[0] == '.' {
				continue
			}
			f := file_t{
				Id:   name,
				Type: "file",
			}

			attr := make(map[string]interface{}, 0)
			f.Attributes = attr

			attr["mod_time"] = file.ModTime().Unix()
			attr["size"] = file.Size()

			if file.IsDir() {
				attr["type"] = "directory"
			} else {
				attr["type"] = "regular file"
			}
			rt = append(rt, f)
		}

		/*
		 * The Content-Length is not set is the buffer length is more than 2048
		 */
		b, err := json.Marshal(hash{
			"data": rt,
		})
		if err != nil {
			return err
		}

		r := c.Response()
		r.Header().Set("Content-Length", strconv.Itoa(len(b)))
		r.Header().Set("Content-Type", "application/json; charset=utf-8")
		r.Write(b)
		return nil
	}

	return c.File(
		path,
		s.Name(),
		true,
	)
}
