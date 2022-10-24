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
PZ_Mod_Data_Dir: "some path"
Font_Color: "#FFF"
Chart_Font_Family: "Impact, sans-serif"
Chart_Font_Size: "22px"
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

Additionally, you can add a refresh in Milliseconds, with the minimum value of 1000 for performance reasons. A 5 second refresh would be `refresh=5000`.  

If you don't know how URLs are built, here are some examples, using `http://192.168.0.333:2501/` as host. **This host
will be different on your machine!**

### Refresh every 3 seconds

```
http://192.168.0.333:2501/?refresh=3000
```

### Show weapon categories, limited to the top 3

```
http://192.168.0.333:2501/?limit=3

with refresh:

http://192.168.0.333:2501/?limit=3&refresh=3000
```

### Show weapon types

```
http://192.168.0.333:2501/?dataType=types

with refresh:

http://192.168.0.333:2501/?dataType=types&refresh=3000
```

### Show weapon types, limited to the top 3

```
http://192.168.0.333:2501/?dataType=types&limit=3

also valid:

http://192.168.0.333:2501/?limit=3&dataType=types

also valid:

http://192.168.0.333:2501/?refresh=3000&limit=3&dataType=types
```

## Bar Chart

Adding `/chart` to your host will provide you with a visual representation of how many Zeds you've killed per day.
So using the example from above, use `http://192.168.0.333:2501/chart` to see the graphic.

### Parameters

In case you don't want to use the bar chart, you can display a line chart instead: `http://192.168.0.333:2501/chart?chartType=line` .
Also, the `refresh` parameter works the same as in the examples above. 