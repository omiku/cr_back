package main

import (
	"context"
	"github.com/qiniu/api.v7/v7/auth/qbox"
	"github.com/qiniu/api.v7/v7/storage"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

var (
	Info, Error *log.Logger
	instruction = os.Args[1]
	pwd = "your path"
	dockerName = "your name"
	passwd = "your passwd"
	accessKey = "your qiniu accessKey"
	secretKey = "your qiniu accessKey"
	bucket = "your qiniu accessKey"
	wg   sync.WaitGroup
)

func init() {
	infoFile, err := os.OpenFile("/var/log/backup.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	errFile, err := os.OpenFile("/var/log/backup.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	if err != nil {
		log.Fatalln("打开日志文件失败：", err)
	}

	Info = log.New(io.MultiWriter(os.Stderr, infoFile), "Info:", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(io.MultiWriter(os.Stderr, errFile), "Error:", log.Ldate|log.Ltime|log.Lshortfile)

	_, err = os.Stat(pwd)
	if err != nil {
		err := os.MkdirAll(pwd, 0600)
		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}
	}
}

func GetTime() (str string) {
	t := time.Now()
	str = t.Format("2006-01-02_15:04")
	return str
}

func Backup(str string) (fList []string) {
	DBFName := "backup_DB_" + str + ".sql"
	FName := "backup_FILE_" + str + ".tar.gz"
	fList = append(fList,DBFName,FName)
  	c1 := "tar -cjf /data/backup/"+FName+" /data/cloudreve  --exclude=cloudreve/aria2-config --exclude=cloudreve/downloads --exclude=cloudreve/mysql"
	//tar -czf /data/backup/"+FName+" /data/cloudreve  --exclude=cloudreve/aria2-config --exclude=cloudreve/downloads --exclude=cloudreve/mysql
	c2 := "docker exec " + dockerName + " sh -c 'exec mysqldump --databases pan -uroot -p\"" + passwd + "\"' > /data/backup/" + DBFName
	cmd := exec.Command("sh", "-c", c1)
	err := cmd.Start()
	Info.Println("Waiting for compression to finish...")
	if err != nil {
		Error.Println(err)
		os.Exit(1)
	}
	cmd2 := exec.Command("sh", "-c", c2)
	_, err = cmd2.Output()
	if err != nil {
		Error.Println(err)
		os.Exit(1)
	}
	_ = cmd.Wait()
	Info.Println("Compression finished ")
	return fList
}

func UpLoad(file string)  {  //上传文件到七牛
	defer wg.Done()
	putPolicy := storage.PutPolicy{
		Scope: bucket,
	}
	mac := qbox.NewMac(accessKey, secretKey)
	upToken := putPolicy.UploadToken(mac)
	cfg := storage.Config{}
	// 空间对应的机房
	cfg.Zone = &storage.Zone_na0
	// 是否使用https域名
	cfg.UseHTTPS = false
	// 上传是否使用CDN上传加速
	cfg.UseCdnDomains = true
	resumeUploader := storage.NewResumeUploader(&cfg)
	ret := storage.PutRet{}
	putExtra := storage.RputExtra{
		ChunkSize : 10 * 1024 * 1024, //10MB
	}
		localFile := pwd + file
		err := resumeUploader.PutFile(context.Background(), &ret, upToken, file, localFile, &putExtra)
		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}
		Info.Println(ret.Key, ret.Hash)
}

func Clean()  {  //删除之前的备份文件
	fileInfoList,err := ioutil.ReadDir(pwd)
	if err != nil {
		Error.Println(err)
		os.Exit(1)
	}
	for i := range fileInfoList {
		err := os.Remove(pwd + fileInfoList[i].Name())  //删除当前文件或目录下的文件或目录名
		if err != nil {
			Error.Println(err)
			os.Exit(1)
		}
	}
	Info.Println("Clean success")
}

func main() {
	if instruction == "clean" {
		Clean()
	} else {
		fList := Backup(GetTime())
		for _,v := range fList{
			wg.Add(1)
			UpLoad(v)
		}
	}
}
