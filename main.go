package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/smtc/glog"
	"github.com/swgloomy/gutil"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

type userList struct {
	Name   string `json:"name"`   //申请人
	Unit   string `json:"unit"`   //依托单位
	School string `json:"school"` //简写
	Field  string `json:"field"`  //研究领域
	Year   string `json:"year"`   //年度
}

func main() {
	gutil.LogInit(false, "./logs")
	defer glog.Close()
	excelPath := "./baike.xlsx"
	excelArray, err := gutil.ReadExcel(excelPath)
	if err != nil {
		glog.Error("main excel get data err! path: %s err: %s \n", excelPath, err.Error())
		return
	}
	var listArray []userList
	for _, sheetArray := range *excelArray {
		for index, item := range sheetArray {
			if index == 0 {
				continue
			}
			listArray = append(listArray, userList{
				Name:   item[1],
				Unit:   item[2],
				School: item[3],
				Field:  item[4],
				Year:   item[5],
			})
		}
		break
	}
	var threadLock sync.WaitGroup
	for index, item := range listArray {
		threadLock.Add(1)
		getUserData(item, &threadLock)
		if index%10 == 0 {
			threadLock.Wait()
		}
	}
	threadLock.Wait()

	fmt.Println("run success!")

	glog.Info("run success!")

}

func getUserData(modal userList, threadLock *sync.WaitGroup) {
	defer threadLock.Done()
	var saveExcel [][]string
	httpUrl := fmt.Sprintf("https://baike.baidu.com/item/%s", modal.Name)
	result, err := http.Get(httpUrl)
	if err != nil {
		glog.Error("getUserData http get err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		return
	}
	defer func() {
		err = result.Body.Close()
		if err != nil {
			glog.Error("getUserData http body close err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		}
	}()

	docQuery, err := goquery.NewDocumentFromReader(result.Body)
	if err != nil {
		glog.Error("getUserData NewDocumentFromReader err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		return
	}
	if strings.Index(docQuery.Find(".lemmaWgt-subLemmaListTitle").Text(), "多义词") >= 0 {
		docQuery.Find(".body-wrapper ul.para-list a").Each(func(i int, elem *goquery.Selection) {
			if strings.Index(elem.Text(), modal.Name) >= 0 && (strings.Index(elem.Text(), modal.School) >= 0 || strings.Index(elem.Text(), modal.Unit) >= 0) {
				href, bo := elem.Attr("href")
				if bo {
					httpUrl = fmt.Sprintf("https://baike.baidu.com%s", href)
					result, err = http.Get(httpUrl)
					if err != nil {
						glog.Error("getUserData http get err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
						return
					}
				}
			}
		})
	}

	docQuery, err = goquery.NewDocumentFromReader(result.Body)
	if err != nil {
		glog.Error("getUserData NewDocumentFromReader err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		return
	}
	saveExcel = append(saveExcel, []string{
		"申请人",
		modal.Name,
	})
	saveExcel = append(saveExcel, []string{
		"依托单位",
		modal.Unit,
	})
	saveExcel = append(saveExcel, []string{
		"简写",
		modal.School,
	})
	saveExcel = append(saveExcel, []string{
		"研究领域",
		modal.Field,
	})
	saveExcel = append(saveExcel, []string{
		"年度",
		modal.Year,
	})
	saveExcel = append(saveExcel, []string{
		"简介",
		docQuery.Find(".lemma-summary").Text(),
	})

	docQuery.Find(".basic-info>dl").Each(func(i int, elem *goquery.Selection) {
		var dataArray [][]string
		elem.Find("dt").Each(func(i int, elem *goquery.Selection) {
			dataArray = append(dataArray, []string{
				elem.Text(),
			})
		})
		elem.Find("dd").Each(func(i int, elem *goquery.Selection) {
			dataArray[i] = append(dataArray[i], elem.Text())
		})
		for _, item := range dataArray {
			saveExcel = append(saveExcel, item)
		}
	})
	docQuery.Find(".para-title.level-2").Each(func(i int, elem *goquery.Selection) {
		var (
			contentArray []string
			nextElector  *goquery.Selection
		)
		contentArray = append(contentArray, strings.Replace(elem.Find("h2").Text(), modal.Name, "", -1))
		for true {
			if nextElector == nil {
				nextElector = elem.Next()
			} else {
				nextElector = nextElector.Next()
			}
			className, bo := nextElector.Attr("class")
			if bo {
				if className != "anchor-list" {
					contentArray = append(contentArray, nextElector.Text())
				} else {
					break
				}
			} else {
				break
			}
		}
		saveExcel = append(saveExcel, contentArray)
	})
	excelSaveData := make(map[string][][]string)
	excelSaveData["sheet"] = saveExcel
	dirPath := fmt.Sprintf("./人物/%s", modal.Name)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		glog.Error("getUserData create dir err! modal: %v dirPath: %s err: %s \n", modal, dirPath, err.Error())
	}
	err = gutil.ExcelSave(&excelSaveData, fmt.Sprintf("%s/%s.xlsx", dirPath, modal.Name))
	if err != nil {
		glog.Error("getUserData ExcelSave err! modal: %v dirPath: %s err: %s \n", modal, dirPath, err.Error())
	}
	imgSrc, bo := docQuery.Find(".wiki-lemma .summary-pic img").Attr("src")
	if bo {
		imgResult, err := http.Get(imgSrc)
		if err != nil {
			glog.Error("getUserData Get img err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
			return
		}
		imgPath := fmt.Sprintf("%s/%s.jpg", dirPath, modal.Name)
		f, err := os.Create(imgPath)
		if err != nil {
			glog.Error("getUserData Create img err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
			return
		}
		_, err = io.Copy(f, imgResult.Body)
		if err != nil {
			glog.Error("getUserData img copy err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
		}
	}

	htmlPath := fmt.Sprintf("%s/%s.html", dirPath, modal.Name)
	result, err = http.Get(httpUrl)
	if err != nil {
		glog.Error("getUserData http get err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		return
	}
	f, err := os.Create(htmlPath)
	if err != nil {
		glog.Error("getUserData Create img err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
		return
	}
	_, err = io.Copy(f, result.Body)
	if err != nil {
		glog.Error("getUserData img copy err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
	}

	glog.Info("getUserData %s run success! \n", modal.Name)
}
