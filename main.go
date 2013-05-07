package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Debug      bool
	Port       string
	ProjectDir string
	Profile    string
}

type content struct {
	ContentHTML template.HTML
	Data        interface{} //content页面绑定的数据，用于layout中template调用传值
}

type Message struct {
	Message string `json:"message"`
}

var Cfg Config

//加载配置文件
func loadConfig(filepath string) {
	fmt.Println("解析配置文件：" + filepath)
	file, err := os.Open(filepath)
	defer file.Close()
	if err != nil {
		log.Fatal("读取配置文件出错", err)
	}
	reader := bufio.NewReader(file)
	bs, _ := ioutil.ReadAll(reader)
	err = json.Unmarshal(bs, &Cfg)
	if err != nil {
		log.Fatal("解析配置文件出错", err)
	}
}

func DateFormat(t time.Time, format string) string {
	return t.Format(format)
}

func TimestampFormat(timestamp int64, format string) string {
	t := time.Unix(timestamp/1000, 0)
	return t.Format(format)
}

func BeginEndFormat(begin time.Time, end time.Time) string {
	nowst := time.Now().Unix()
	beginst := begin.Unix()
	endst := end.Unix()
	if nowst < beginst {
		return "开始时间：" + DateFormat(begin, "2006-01-02 15:04")
	} else if nowst >= beginst && nowst <= endst {
		r := endst - nowst
		d := r / 86400
		h := r % 86400 / 3600
		if d == 0 && h == 0 {
			h = 1
		}
		result := ""
		if d > 0 {
			result = result + strconv.Itoa(int(d)) + "天"
		}
		if h > 0 {
			result = result + strconv.Itoa(int(h)) + "小时"
		}
		return result + "后结束"
	}
	return "结束时间：" + DateFormat(end, "2006-01-02 15:04")
}

func intplus(input int, n int) int {
	return input + n
}

func parseTemplate(file string, data interface{}) (t *template.Template, c []byte, err error) {
	var buf bytes.Buffer
	t = template.New(filepath.Base(file))
	t.Funcs(template.FuncMap{
		"dateFormat":      DateFormat,
		"timestampFormat": TimestampFormat,
		"intplus":         intplus,
		"beginEndFormat":  BeginEndFormat})
	t, err = t.ParseFiles("views/base.html", file)
	if err != nil {
		return nil, nil, err
	}
	err = t.ExecuteTemplate(&buf, filepath.Base(file), data)
	if err != nil {
		return nil, nil, err
	}
	return t, buf.Bytes(), nil
}

func getPage(file string, data interface{}) []byte {
	var buf bytes.Buffer
	t, c, err := parseTemplate(file, data)
	if err != nil {
		log.Println(err)
	}
	err = t.ExecuteTemplate(&buf, filepath.Base("views/base.html"), content{template.HTML(c), data})
	if err != nil {
		log.Println(err)
	}
	return buf.Bytes()
}

func serveJson(w http.ResponseWriter, v interface{}) {
	content, err := json.Marshal(v)
	if err != nil {
		log.Print(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	if Cfg.Debug {
		w.Header().Set("Access-Control-Allow-Origin", "*") //跨域
	}
	w.Write(content)
}

func validateRequired(r *http.Request, names ...string) error {
	for _, name := range names {
		if r.FormValue(name) == "" {
			return errors.New(name + "必填")
		}
	}
	return nil
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Write(getPage("views/home.html", nil))
}

func outParse(out string) (res []string) {
	res = strings.Split(strings.Replace(out, Cfg.ProjectDir, "", -1), "\n")
	return
}

func svnUpdate(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("svn", "update", Cfg.ProjectDir)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		serveJson(w, Message{err.Error()})
		return
	}
	serveJson(w, outParse(out.String()))
}

func svnStatus(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("svn", "status", Cfg.ProjectDir)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		serveJson(w, Message{err.Error()})
		return
	}
	serveJson(w, outParse(out.String()))
}

func restart(w http.ResponseWriter, r *http.Request) {
	var out bytes.Buffer
	cmd := exec.Command("mvn", "jetty:stop")
	cmd.Dir = Cfg.ProjectDir
	cmd.Stdout = &out
	err := cmd.Run()
	if err == nil {
		log.Println("stop success!")
		if Cfg.Profile == "" {
			cmd = exec.Command("mvn", "jetty:run")
		} else {
			cmd = exec.Command("mvn", "jetty:run", "-P"+Cfg.Profile)
		}
		cmd.Dir = Cfg.ProjectDir
		cmd.Stdout = &out
		//TODO 后台运行并返回结果，达到&效果
		err = cmd.Start() //Run()
		if err == nil {
			return
		}
	}
	log.Println(out.String())
	log.Println(err)
	w.WriteHeader(http.StatusInternalServerError)
	serveJson(w, Message{err.Error()})
}

func main() {
	wd, _ := os.Getwd()
	loadConfig(path.Join(wd, "config.json")) //加载配置文件

	//静态文件路由
	http.Handle("/img/", http.FileServer(http.Dir("static")))
	http.Handle("/js/", http.FileServer(http.Dir("static")))
	http.Handle("/css/", http.FileServer(http.Dir("static")))

	//设置访问的路由
	http.HandleFunc("/", home)
	http.HandleFunc("/update", svnUpdate)
	http.HandleFunc("/status", svnStatus)
	http.HandleFunc("/restart", restart)

	err := http.ListenAndServe(":"+Cfg.Port, nil) //设置监听的端口
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
