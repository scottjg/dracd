#!/usr/bin/env ruby

binary_files = %w( connecting.png waiting_for_video_signal.png )
noroute_files = %w( connect.html )
Dir.chdir("novnc")
files = %w( include/util.js include/webutil.js include/base64.js
	include/websock.js include/des.js include/keysymdef.js
	include/keyboard.js include/input.js include/display.js
	include/jsunzip.js include/rfb.js include/keysym.js include/base.css )

routes = {}

puts "package main"
puts ""
puts "import ("
puts "	\"github.com/gorilla/mux\""
puts "	\"io\""
puts "	\"net/http\""
puts ")"
puts ""

files.each do |filename|
  content = File.open(filename).read.gsub(/`/,"'")
  varname = filename.gsub(/[\.\/]/, '_')
  puts "var #{varname} = \`#{content}\`"
  puts ""
  puts "func #{varname}Handler(w http.ResponseWriter, r *http.Request) {"
  puts "	w.Header().Set(\"Content-Type\", \"text/css\")" if filename =~ /\.css$/
  puts "	w.Header().Set(\"Content-Type\", \"application/javascript\")" if filename =~ /\.js$/
  puts "	io.WriteString(w, #{varname})"
  puts "}"
  puts ""

  routes[filename] = varname
end

Dir.chdir("..")
noroute_files.each do |filename|
  content = File.open(filename).read.gsub(/`/,"'")
  varname = filename.gsub(/[\.\/]/, '_')
  puts "var #{varname} = \`#{content}\`"
  puts ""
end

binary_files.each do |filename|
  content = File.open(filename).read
  varname = filename.gsub(/[\.\/]/, '_')
  print "var #{varname} = []byte{"
  content.each_byte do |b|
    print "0x%02x, " % b
  end
  puts "}"
  puts ""
end

puts "func routeAssets(r *mux.Router) {"
routes.keys.each do |filename|
	puts "	r.HandleFunc(\"/#{filename}\", #{routes[filename]}Handler)"
end
puts "}"
