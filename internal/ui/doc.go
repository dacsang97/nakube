// Package ui holds reusable window widgets. It does not know about
// Kubernetes. The shell lives in desktop. Each screen lives in its own
// views package. Widgets stay here so a screen cannot reach into another
// screen to borrow a control.
package ui
