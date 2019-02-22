# m3u8-downloader

golang 多线程下载直播流m3u8格式的视屏，跨平台（建议unix上运行）

> 以下载岛国小电影哦  
> 可以下载岛国小电影哦  
> 可以下载岛国小电影哦    
> 重要的事情说三遍......

### 运行

`go run m3u8.go -u "m3u8的url" -o "下载的电影名[默认：url截取的名字]" -n 80`


### 功能介绍

1. 多线程下载m3u8的ts片段（加密的同步解密）
2. 合并下载的ts文件


### 可能遇到的异常、解决方法 (看@todo）

1. 下载失败的情况【详情请见代码里面：get_host方法】

```golang

func get_host(Url string) string {
	u, err := url.Parse(Url)
	check(err)
	//@todo [see here 选择注释以下其中之一，主要用来构建不同的ts_url地址]
	return u.Scheme + "://" + u.Host + path.Dir(u.Path)
	return u.Scheme + "://" + u.Host
}
```

2. windows端运行下载的视屏顺序错乱

```golang
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
//@todo [dos命令不熟，可能导致顺序乱，dos大神可仿照linux的合并方法unix_merge_file做调整，或者手动合并 O(∩_∩)O~~]
func win_merge_file(path string) {
	os.Chdir(path)
	ExecShell("del /Q ad*.ts")
	ExecShell("copy /b *.ts new.tmp")
	//ExecShell("del /Q *.ts")
	ExecShell("del /Q *.mp4")
	os.Rename("new.tmp", "new.mp4")
}
```
