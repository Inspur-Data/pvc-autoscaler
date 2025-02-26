module pvc-operator

go 1.13

require (
	github.com/astaxie/beego v1.12.1
	github.com/go-logr/logr v1.2.3
	github.com/henrylee2cn/mahonia v0.0.0-20150715080413-be6deb105fbc
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/robfig/cron v1.2.0
	github.com/shiena/ansicolor v0.0.0-20200904210342-c7312218db18 // indirect
	github.com/sirupsen/logrus v1.8.1
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	k8s.io/api v0.26.10
	k8s.io/apimachinery v0.26.10
	k8s.io/client-go v0.26.10
	sigs.k8s.io/controller-runtime v0.14.7
)
