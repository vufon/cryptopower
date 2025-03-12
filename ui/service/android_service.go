//go:build android
// +build android

package service

/*
#cgo LDFLAGS: -llog
#include <jni.h>
*/

import "C"
import (
	giouiApp "gioui.org/app"
	"git.wow.st/gmp/jni"
)

const (
	serviceClass = "org/gioui/x/service/KeepAliveService"
)

func RegisterKeepAliveService() {
	jni.Do(jni.JVMFor(giouiApp.JavaVM()), func(env jni.Env) error {
		appCtx := jni.Object(giouiApp.AppContext())
		classLoader := jni.ClassLoaderFor(env, appCtx)
		serviceFullClass, err := jni.LoadClass(env, classLoader, serviceClass)
		if err != nil {
			return err
		}
		// call startService
		startServiceMethod := jni.GetStaticMethodID(env, serviceFullClass, "startService", "(Landroid/content/Context;)V")
		jni.CallStaticVoidMethod(env, serviceFullClass, startServiceMethod, jni.Value(giouiApp.AppContext()))
		return nil
	})
}
