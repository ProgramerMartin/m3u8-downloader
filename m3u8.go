//author:lychao8<lychao_vip@163.com>
//date:2109-02-18
package main

import (
	"flag"
	"fmt"
	"github.com/levigross/grequests"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
	"os/exec"
	"bytes"
	"net/url"
	"path"
	"runtime"
	"sync"
	"crypto/aes"
	"crypto/cipher"
	"io/ioutil"
)

const (
	HEAD_TIMEOUT = 10 * time.Second
)

var (
	urlFlag = flag.String("u", "", "url of m3u8")
	nFlag   = flag.Int("n", 80, "max goroutines num")
	oFlag   = flag.String("o", "new", "name of the download  movie")

	logger *log.Logger
	ro     = &grequests.RequestOptions{
		UserAgent:      "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1; SV1; AcooBrowser; .NET CLR 1.1.4322; .NET CLR 2.0.50727)",
		RequestTimeout: HEAD_TIMEOUT,
		Headers: map[string]string{
			"Connection":      "keep-alive",
			"Accept":          "*/*",
			"Accept-Encoding": "br, gzip, deflate",
			"Accept-Language": "zh-Hans;q=1",
		},
	}
)

func init() {
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
}

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func AesEncrypt(origData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

func AesDecrypt(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS7UnPadding(origData)
	return origData, nil
}

func check(e error) {
	if e != nil {
		logger.Panic(e)
	}
}

func get_m3u8_body(Url string) string {
	r, err := grequests.Get(Url, ro)
	check(err)
	return r.String()
}

//获取m3u8加密的密钥
func get_m3u8_key(html string) (key string) {
	lines := strings.Split(html, "\n")
	key = ""
	for _, line := range lines {
		if strings.Contains(line, "#EXT-X-KEY") {
			uri_pos := strings.Index(line, "URI")
			quotation_mark_pos := strings.LastIndex(line, "\"")
			key_url := strings.Split(line[uri_pos:quotation_mark_pos], "\"")[1]
			if !strings.Contains(line, "http") {
				key_url = fmt.Sprintf("%s/%s", get_host(*urlFlag), key_url)
			}
			logger.Println("key_url:", key_url)
			res, err := grequests.Get(key_url, ro)
			check(err)
			if res.StatusCode == 200 {
				key = res.String()
			}
		}
	}
	return
}

func get_url_list(host, body string) (list []string) {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && line != "" {
			if strings.HasPrefix(line, "http") {
				list = append(list, line)
			} else {
				list = append(list, fmt.Sprintf("%s/%s", host, line))
			}
		}
	}
	return
}

//去掉字符串所有的非数字的字符
func TrimStr2int(str string) string {
	newStr := strings.TrimRight(str, ".ts")
	re3, _ := regexp.Compile("[a-zA-Z_]+")
	intStr := re3.ReplaceAllString(newStr, "")
	return intStr
}

//下载ts文件
func download_ts_file(ts_url, download_dir, key string, retries uint) {
	logger.Println("start ts_url:", ts_url, time.Now().Second())

	file_name := ts_url[strings.LastIndex(ts_url, "/"):]
	file_name_new := TrimStr2int(file_name) + ".ts"
	curr_path := fmt.Sprintf("%s%s", download_dir, file_name_new)
	if isExist, _ := PathExists(curr_path); isExist {
		logger.Println("[warn]: file already exist")
		return
	}

	res, err := grequests.Get(ts_url, ro)
	if err != nil || !res.Ok {
		if retries > 0 {
			logger.Printf("[warn]Retry:%d, %s", retries-1, ts_url)
			time.Sleep(2 * time.Second)
			download_ts_file(ts_url, download_dir, key, retries-1)
			return
		} else {
			return
		}
	}

	if key == "" {
		res.DownloadToFile(curr_path)
	} else {
		//若加密，解密ts文件 aes 128 cbc pack5
		origData, err := AesDecrypt(res.Bytes(), []byte(key))
		if err != nil {
			download_ts_file(ts_url, download_dir, key, retries-1)
			return
		}
		ioutil.WriteFile(curr_path, origData, 0666)
	}
}

//执行shell
func ExecShell(s string) {
	cmd := exec.Command("/bin/bash", "-c", s)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", out.String())
}

//unix合并文件
func unix_merge_file(path string) {
	os.Chdir(path)
	ExecShell("rm -rf ad*.ts")
	cmd := `ls  *.ts |sort -t "\." -k 1 -n |awk '{print $0}' |xargs -n 1 -I {} bash -c "cat {} >> new.tmp"`
	ExecShell(cmd)
	ExecShell("rm -rf *.ts")
	os.Rename("new.tmp", "new.mp4")
}

//windows合并文件
//@todo [dos命令不熟，可能导致顺序乱，dos大神可仿照linux的合并方法unix_merge_file做调整]
func win_merge_file(path string) {
	os.Chdir(path)
	ExecShell("del /Q ad*.ts")
	ExecShell("copy /b *.ts new.tmp")
	//ExecShell("del /Q *.ts")
	ExecShell("del /Q *.mp4")
	os.Rename("new.tmp", "new.mp4")
}

//判断文件是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func get_host(Url string) string {
	u, err := url.Parse(Url)
	check(err)
	//@todo [see here 选择注释以下其中之一，主要用来构建不同的ts_url地址]
	return u.Scheme + "://" + u.Host + path.Dir(u.Path)
	return u.Scheme + "://" + u.Host
}

func Run() {
	logger.Println("============================ 功能：多线程下载直播流m3u8的视屏（ts+合并） ========================")
	logger.Println("============================ 下载失败的情况【详情请见代码里面：get_host方法】 ========================")
	runtime.GOMAXPROCS(runtime.NumCPU())
	now := time.Now()

	flag.Parse()

	if !strings.HasPrefix(*urlFlag, "http") || !strings.Contains(*urlFlag, "m3u8") {
		flag.Usage()
		return
	}
	Url := *urlFlag
	maxGoroutines := *nFlag

	//Url = "https://cn1.bb997.me/sehua/mywife-1458.m3u8"

	pwd, _ := os.Getwd()
	//pwd = "/Users/chao/Desktop"

	movieDir := *oFlag
	if *oFlag == "new" {
		movieDir = path.Base(Url)
	}
	download_dir := pwd + "/movie/" + string(movieDir) + time.Now().Format("0601021504")
	if isExist, _ := PathExists(download_dir); !isExist {
		os.MkdirAll(download_dir, os.ModePerm)
	}

	host := get_host(Url)
	body := get_m3u8_body(Url)

	key := get_m3u8_key(body)
	logger.Printf("key: %s", key)

	url_list := get_url_list(host, body)
	logger.Println("url_list:", url_list)

	var wg sync.WaitGroup
	limiter := make(chan struct{}, maxGoroutines)
	for _, ts_url := range url_list {
		wg.Add(1)
		limiter <- struct{}{}
		go func(ts_url, download_dir, key string, retryies uint) {
			defer func() {
				wg.Done()
				<-limiter
				logger.Println("from ch", time.Now().Second())
			}()
			download_ts_file(ts_url, download_dir, key, 3)
			return
		}(ts_url, download_dir, key, 3)
	}

	wg.Wait()
	logger.Printf("下载完成，耗时:%#vs\n", time.Now().Sub(now).Seconds())

	switch runtime.GOOS {
	case "darwin", "linux":
		unix_merge_file(download_dir)
	case "windows":
		win_merge_file(download_dir)
	default:
		unix_merge_file(download_dir)
	}

	logger.Printf("任务完成，耗时:%#vs\n", time.Now().Sub(now).Seconds())
}

func main() {
	Run()
}
