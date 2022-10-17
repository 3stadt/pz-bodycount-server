# PZ bodycount server

This project is built to be used together with the project zomboid mod available at
[3stadt/BodyCount](https://github.com/3stadt/BodyCount).

It was built for [TwistOnFire](https://www.twitch.tv/twistonfire) and is not intended to be used by the public. Still,
if you feel like it, go ahead.

## Configuration

There is a `config.txt` file alongside the executable after compilation.

In this file, you can configure the host to listen on and your own custom HTML file if you want to use one.   
If no custom HTML file ist specified, the *included* `template.gohtml` is used. On build, the current template is baked
into the executable in order to always have a fallback.

Here is an example of a full config file:

```
Listen_Address: ":2501"
Template_File: "my_template.gohtml"
PZ_Mod_Data_Dir: "some path"
```

Where `PZ_Mod_Data_Dir` is something you would only need to change if you know exactly what you are doing.
The format of the text file is read as **YAML** - So keep the formatting stuff like `:` and `"` or things will break.

## Parameters

The program will show an IP address it listens to.   
If you configured just a port as in the example below, this will be you outgoing interface, meaning the network adress
you connect to the internet with.

In any case, after you copied the URL you can add parameters. By default, the weapon categories are displayed in
descending order.
You can add `?dataType=types` to the URL to show weapon types instead. Also, you can use `?limit=5` to show only the
first 5 entries, change 5 to whatever you want.

If you don't know how URLs are built, here are some examples, using `http://192.168.0.333:2501/` as host. **This host
will be different on your machine!**

### Show weapon categories, limited to the top 3

```
http://192.168.0.333:2501/?limit=3
```

### Show weapon types

```
http://192.168.0.333:2501/?dataType=types
```

### Show weapon types, limited to the top 3

```
http://192.168.0.333:2501/?dataType=types&limit=3

also valid:

http://192.168.0.333:2501/?limit=3&dataType=types
```