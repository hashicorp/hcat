/*
The Hashicat library.

This library provides a means to fetch data managed by external services and
render templates using that data. It also enables monitoring those services for
data changes to trigger updates to the templates.

A simple example of how you might use this library to generate the contents of
a single template, waiting for all its dependencies (external data) to be
fetched and filled in, then have that content returned.
*/
package hcat
