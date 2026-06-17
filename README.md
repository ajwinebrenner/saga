# saga

A simple toolkit for creating interactive stories and text-based adventures.

## Why saga?

The goal of this package is to provide a consistent yet flexible way to organise a collection of story scenes.
I wanted to be able to traverse from one scene to another with a finite number of options, similar to a directed graph.
However, I wanted the whole structure to be declarative but still enforce all references to other nodes be safe and valid.
This would seem to necessitate some kind of build step to handle the heavy lifting. Saga aims to solve this problem.

## Structure

This package provides a structure of scenes called a world, built from declarative input.
Each scene can represent a location or a point in time; a story beat or moment of decision.
Scenes have routes (akin to graph edges) that when followed, lead to other scenes and consequences.

Scenes may also contain other scenes so that all choices on a parent scene are available to all sub-scenes.
This grouping of scenes along with features such as conditional choices and overriding outcomes
makes traversal of the world more dynamic than traditional pen and paper interactive fiction.

## State

In order to implement dynamic descriptions, outcomes, and progression through the narrative,
the world must be able to read and update arbitrary data structures, referred to as systems.

Scenes have some fields that may not necessarily be constant. These fields use `DynVal[T]`, a dynamic value.
Dynamic values can depend on 0 or more systems as defined by the package consumer, constituting shared state.
Two helper functions create dynamic values that safely check the needed systems while building the world:

- `Just`: Returns just the value of the argument with no system dependencies
- `Dyn`: Accepts a function with a single argument, a system or an anonymous struct of 1 or more systems

When building the world, any dynamic values requiring systems not present in the world will cause an error.
Additionally, scenes containing any reference to another scene not present will also cause an error while building.
This fulfils the original goal of this package, to create a safe collection of scenes that won't error during traversal. 

## World

The world returned from `Build` has a small set of methods for interacting with world state.
Included methods can read the current description of the world, list available choices, and make a choice to progress.
The way an end user might interact with the world is flexible, the simplest way being a purely text-based REPL.
